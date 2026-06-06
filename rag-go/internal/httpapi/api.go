package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"llmentor/rag-go/internal/cleaner"
	"llmentor/rag-go/internal/config"
	"llmentor/rag-go/internal/dashscope"
	"llmentor/rag-go/internal/document"
	"llmentor/rag-go/internal/es"
	miniostore "llmentor/rag-go/internal/minio"
	"llmentor/rag-go/internal/reader"
	"llmentor/rag-go/internal/rerank"
	"llmentor/rag-go/internal/splitter"
	"llmentor/rag-go/internal/vectorstore"
)

type App struct {
	cfg    *config.Config
	reader *reader.Factory
	llm    *dashscope.Client
	vector *vectorstore.Store
	es     *es.Client
	minio  *miniostore.Client
}

func New(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	vector, err := vectorstore.New(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	minioClient, err := miniostore.New(cfg.MinIO)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:    cfg,
		reader: reader.NewFactory(cfg.Reader.TikaURL),
		llm:    dashscope.New(cfg.DashScope),
		vector: vector,
		es:     es.New(cfg.Elasticsearch),
		minio:  minioClient,
	}, nil
}

func (a *App) Init(ctx context.Context) error {
	if err := a.es.EnsureIndex(ctx); err != nil {
		return err
	}
	return nil
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.health)
	mux.HandleFunc("/rag/read", a.read)
	mux.HandleFunc("/rag/chunker", a.chunker)
	mux.HandleFunc("/rag/split", a.split)
	mux.HandleFunc("/rag/splitRecursive", a.splitRecursive)
	mux.HandleFunc("/rag/splitSentence", a.splitSentence)
	mux.HandleFunc("/rag/splitParent", a.splitParent)
	mux.HandleFunc("/rag/embedding/test", a.embeddingTest)
	mux.HandleFunc("/rag/embedding/embed", a.embeddingEmbed)
	mux.HandleFunc("/rag/es/write", a.esWrite)
	mux.HandleFunc("/rag/es/search", a.esSearch)
	mux.HandleFunc("/rag/retriever/query", a.retrieverQuery)
	mux.HandleFunc("/rag/retriever/retrieve", a.retrieverRetrieve)
	mux.HandleFunc("/rag/retriever/retrieveAdvisor", a.retrieverRetrieve)
	mux.HandleFunc("/rag/hybrid/write", a.hybridWrite)
	mux.HandleFunc("/rag/hybrid/searchFromEs", a.hybridSearchFromES)
	mux.HandleFunc("/rag/hybrid/searchFromVector", a.hybridSearchFromVector)
	mux.HandleFunc("/rag/hybrid/searchFromHybrid", a.hybridSearchFromHybrid)
	mux.HandleFunc("/rag/hybrid/chatToHybrid", a.hybridChatToHybrid)
	mux.HandleFunc("/rag/files/upload", a.fileUpload)
	mux.HandleFunc("/rag/files/download-url/", a.fileDownloadURL)
	return mux
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"status": "ok", "service": "rag-go"})
}

func (a *App) read(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readAndClean(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var b strings.Builder
	for _, doc := range docs {
		b.WriteString(doc.Text)
		b.WriteString("========================")
	}
	writeText(w, b.String())
}

func (a *App) chunker(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readAndClean(r)
	if err != nil {
		writeError(w, err)
		return
	}
	_ = splitter.OverlapDocuments(docs, 100, 5)
	writeText(w, "success")
}

func (a *App) split(w http.ResponseWriter, r *http.Request) {
	a.chunker(w, r)
}

func (a *App) splitRecursive(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readFromPath(param(r, "filePath"))
	if err != nil {
		writeError(w, err)
		return
	}
	_ = splitter.RecursiveDocuments(docs, 300, 0, []string{"\n\n", "\n"})
	writeText(w, "success")
}

func (a *App) splitSentence(w http.ResponseWriter, r *http.Request) {
	text := param(r, "text")
	if text == "" {
		text = "Harry Potter is a series of seven fantasy novels written by British author J. K. Rowling. The novels chronicle the lives of a young wizard. The series was originally published in English by Bloomsbury."
	}
	writeJSON(w, splitter.SentenceText(text, 100))
}

func (a *App) splitParent(w http.ResponseWriter, r *http.Request) {
	text := param(r, "text")
	if text == "" {
		text = "# title\ncontent\n## subtitle\nmore content"
	}
	writeJSON(w, splitter.MarkdownHeaderDocuments(text))
}

func (a *App) embeddingTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, err := a.llmForRequest(r).Embeddings(ctx, []string{"test"})
	if err != nil {
		writeError(w, err)
		return
	}
	writeText(w, "success")
}

func (a *App) embeddingEmbed(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readAndClean(r)
	if err != nil {
		writeError(w, err)
		return
	}
	docs = withCategory(docs, param(r, "category"))
	chunks := cleaner.CleanDocuments(splitter.OverlapDocuments(docs, 1000, 50))
	if err := a.embedAndStore(r.Context(), chunks, a.llmForRequest(r)); err != nil {
		writeError(w, err)
		return
	}
	writeText(w, "success")
}

func (a *App) esWrite(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readAndClean(r)
	if err != nil {
		writeError(w, err)
		return
	}
	docs = withCategory(docs, param(r, "category"))
	chunks := splitter.OverlapDocuments(docs, 200, 50)
	if err := a.es.BulkIndex(r.Context(), chunks); err != nil {
		writeError(w, err)
		return
	}
	writeText(w, "success")
}

func (a *App) esSearch(w http.ResponseWriter, r *http.Request) {
	docs, err := a.es.SearchByKeyword(r.Context(), param(r, "keyword"), intParam(r, "topK", 5), param(r, "category"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, docs)
}

func (a *App) retrieverQuery(w http.ResponseWriter, r *http.Request) {
	docs, err := a.vectorSearch(r.Context(), param(r, "query"), floatParam(r, "threshold", 0.5), intParam(r, "topK", 5), param(r, "category"), a.llmForRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	var b strings.Builder
	for _, doc := range docs {
		b.WriteString(doc.Text)
		b.WriteString("\n==========================")
	}
	writeText(w, b.String())
}

func (a *App) retrieverRetrieve(w http.ResponseWriter, r *http.Request) {
	query := param(r, "query")
	llm := a.llmForRequest(r)
	docs, err := a.vectorSearch(r.Context(), query, floatParam(r, "threshold", 0.5), intParam(r, "topK", 5), param(r, "category"), llm)
	if err != nil {
		writeError(w, err)
		return
	}
	answer, err := a.answerFromDocuments(r.Context(), query, docTexts(docs), llm)
	if err != nil {
		writeError(w, err)
		return
	}
	writeText(w, answer)
}

func (a *App) hybridWrite(w http.ResponseWriter, r *http.Request) {
	docs, err := a.readFromPath(param(r, "filePath"))
	if err != nil {
		writeError(w, err)
		return
	}
	docs = withCategory(docs, param(r, "category"))
	chunks := splitter.RecursiveDocuments(docs, 100, 0, []string{"\u3002"})
	if err := a.es.BulkIndex(r.Context(), chunks); err != nil {
		log.Printf("es bulk index skipped: %v", err)
	}
	if err := a.embedAndStore(r.Context(), chunks, a.llmForRequest(r)); err != nil {
		writeError(w, err)
		return
	}
	writeText(w, "success")
}

func (a *App) hybridSearchFromES(w http.ResponseWriter, r *http.Request) {
	a.esSearch(w, r)
}

func (a *App) hybridSearchFromVector(w http.ResponseWriter, r *http.Request) {
	docs, err := a.vectorSearch(r.Context(), param(r, "keyword"), floatParam(r, "threshold", 0.5), intParam(r, "topK", 5), param(r, "category"), a.llmForRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, docs)
}

func (a *App) hybridSearchFromHybrid(w http.ResponseWriter, r *http.Request) {
	contents, err := a.hybridContents(r.Context(), param(r, "keyword"), param(r, "category"), intParam(r, "topK", 5), boolParam(r, "rerank", true), a.llmForRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, contents)
}

func (a *App) hybridChatToHybrid(w http.ResponseWriter, r *http.Request) {
	llm := a.llmForRequest(r)
	keyword := param(r, "keyword")
	rewritePrompt := "Rewrite the user question to be specific and detailed. If there are typos, correct them. Return only the rewritten question.\n\nUser question:\n" + keyword
	newQuestion, err := llm.Chat(r.Context(), "", rewritePrompt)
	if err != nil {
		log.Printf("query rewrite failed, using original: %v", err)
		newQuestion = keyword
	}
	contents, err := a.hybridContents(r.Context(), newQuestion, param(r, "category"), intParam(r, "topK", 5), boolParam(r, "rerank", true), llm)
	if err != nil {
		writeError(w, err)
		return
	}
	answer, err := a.answerFromDocuments(r.Context(), newQuestion, contents, llm)
	if err != nil {
		writeError(w, err)
		return
	}
	writeText(w, answer)
}

func (a *App) fileUpload(w http.ResponseWriter, r *http.Request) {
	objectName, err := a.minio.UploadFromURL(r.Context(), param(r, "fileUrl"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeText(w, "upload success: "+objectName)
}

func (a *App) fileDownloadURL(w http.ResponseWriter, r *http.Request) {
	objectName := strings.TrimPrefix(r.URL.Path, "/rag/files/download-url/")
	if objectName == "" {
		writeError(w, errors.New("objectName is required"))
		return
	}
	url, err := a.minio.PresignedURL(r.Context(), objectName)
	if err != nil {
		writeError(w, err)
		return
	}
	writeText(w, url)
}

func (a *App) readAndClean(r *http.Request) ([]document.Document, error) {
	docs, err := a.readFromPath(param(r, "filePath"))
	if err != nil {
		return nil, err
	}
	return cleaner.CleanDocuments(docs), nil
}

func (a *App) readFromPath(filePath string) ([]document.Document, error) {
	if filePath == "" {
		return nil, errors.New("filePath is required")
	}
	return a.reader.Read(filePath)
}

func (a *App) llmForRequest(r *http.Request) *dashscope.Client {
	return a.llm.WithAPIKey(r.Header.Get("X-T2T-DashScope-Key"))
}

func (a *App) embedAndStore(ctx context.Context, docs []document.Document, llm *dashscope.Client) error {
	texts := docTexts(docs)
	vectors, err := llm.Embeddings(ctx, texts)
	if err != nil {
		return err
	}
	return a.vector.Add(ctx, docs, vectors)
}

func (a *App) vectorSearch(ctx context.Context, query string, threshold float64, topK int, category string, llm *dashscope.Client) ([]document.Document, error) {
	if query == "" {
		return nil, errors.New("query/keyword is required")
	}
	vectors, err := llm.Embeddings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, errors.New("empty embedding")
	}
	return a.vector.Search(ctx, vectors[0], threshold, topK, category)
}

func (a *App) hybridContents(ctx context.Context, keyword string, category string, topK int, useRerank bool, llm *dashscope.Client) ([]string, error) {
	vectorDocs, vectorErr := a.vectorSearch(ctx, keyword, 0.5, topK, category, llm)
	esDocs, esErr := a.es.SearchByKeyword(ctx, keyword, topK, category)
	if vectorErr != nil && esErr != nil {
		return nil, fmt.Errorf("hybrid search failed: vector: %v; es: %v", vectorErr, esErr)
	}
	if vectorErr != nil {
		vectorDocs = nil
	}
	if esErr != nil {
		esDocs = nil
	}
	rrf := rerank.RRF(vectorDocs, esDocs, 20)
	if useRerank {
		if reranked, err := llm.Rerank(ctx, keyword, rrf, topK); err == nil && len(reranked) > 0 {
			return reranked, nil
		}
	}
	if topK <= 0 {
		topK = 5
	}
	if topK > len(rrf) {
		topK = len(rrf)
	}
	return rrf[:topK], nil
}

func (a *App) answerFromDocuments(ctx context.Context, question string, contents []string, llm *dashscope.Client) (string, error) {
	prompt := fmt.Sprintf(`Answer the user question based only on the following reference documents.
If the documents do not contain relevant information, say that no relevant information was found.

Reference documents:
%s

User question: %s`, strings.Join(contents, "\n\n========= document separator =========\n\n"), question)
	return llm.Chat(ctx, "", prompt)
}

func withCategory(docs []document.Document, category string) []document.Document {
	category = strings.TrimSpace(category)
	if category == "" {
		return docs
	}
	for i := range docs {
		if docs[i].Metadata == nil {
			docs[i].Metadata = map[string]any{}
		}
		docs[i].Metadata["category"] = category
	}
	return docs
}

func docTexts(docs []document.Document) []string {
	texts := make([]string, 0, len(docs))
	for _, doc := range docs {
		texts = append(texts, doc.Text)
	}
	return texts
}

func param(r *http.Request, key string) string {
	_ = r.ParseMultipartForm(64 << 20)
	if v := r.FormValue(key); v != "" {
		return v
	}
	return r.URL.Query().Get(key)
}

func intParam(r *http.Request, key string, fallback int) int {
	v := param(r, key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolParam(r *http.Request, key string, fallback bool) bool {
	v := param(r, key)
	if v == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func floatParam(r *http.Request, key string, fallback float64) float64 {
	v := param(r, key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func writeText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
