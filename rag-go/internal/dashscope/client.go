package dashscope

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llmentor/rag-go/internal/config"
)

type Client struct {
	apiKey              string
	compatibleBaseURL   string
	nativeBaseURL       string
	embeddingModel      string
	embeddingDimensions int
	chatModel           string
	rerankModel         string
	http                *http.Client
}

const maxEmbeddingBatchSize = 10

func New(cfg config.DashScopeConfig) *Client {
	return &Client{
		apiKey:              cfg.APIKey,
		compatibleBaseURL:   strings.TrimRight(cfg.CompatibleBaseURL, "/"),
		nativeBaseURL:       strings.TrimRight(cfg.NativeBaseURL, "/"),
		embeddingModel:      cfg.EmbeddingModel,
		embeddingDimensions: cfg.EmbeddingDimensions,
		chatModel:           cfg.ChatModel,
		rerankModel:         cfg.RerankModel,
		http:                &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) WithAPIKey(apiKey string) *Client {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return c
	}
	clone := *c
	clone.apiKey = apiKey
	return &clone
}

func (c *Client) Embeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if c.apiKey == "" {
		return nil, errors.New("DASHSCOPE_API_KEY is empty")
	}
	if len(texts) == 0 {
		return nil, nil
	}
	if len(texts) > maxEmbeddingBatchSize {
		out := make([][]float64, 0, len(texts))
		for start := 0; start < len(texts); start += maxEmbeddingBatchSize {
			end := start + maxEmbeddingBatchSize
			if end > len(texts) {
				end = len(texts)
			}
			batch, err := c.embeddingsOnce(ctx, texts[start:end])
			if err != nil {
				return nil, fmt.Errorf("embedding batch %d-%d failed: %w", start+1, end, err)
			}
			out = append(out, batch...)
		}
		return out, nil
	}
	return c.embeddingsOnce(ctx, texts)
}

func (c *Client) embeddingsOnce(ctx context.Context, texts []string) ([][]float64, error) {
	body := map[string]any{
		"model": c.embeddingModel,
		"input": texts,
	}
	if c.embeddingDimensions > 0 {
		body["dimensions"] = c.embeddingDimensions
	}

	var resp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error any `json:"error,omitempty"`
	}
	if err := c.postJSON(ctx, c.compatibleBaseURL+"/embeddings", body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: got %d want %d", len(resp.Data), len(texts))
	}
	out := make([][]float64, len(resp.Data))
	for i := range resp.Data {
		out[i] = resp.Data[i].Embedding
	}
	return out, nil
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("DASHSCOPE_API_KEY is empty")
	}
	messages := []map[string]string{}
	if strings.TrimSpace(system) != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": user})
	body := map[string]any{
		"model":       c.chatModel,
		"messages":    messages,
		"temperature": 0.7,
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := c.postJSON(ctx, c.compatibleBaseURL+"/chat/completions", body, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("chat response has no choices")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *Client) Rerank(ctx context.Context, query string, docs []string, topN int) ([]string, error) {
	if c.apiKey == "" {
		return nil, errors.New("DASHSCOPE_API_KEY is empty")
	}
	if len(docs) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(docs) {
		topN = len(docs)
	}
	body := map[string]any{
		"model": c.rerankModel,
		"input": map[string]any{
			"query":     query,
			"documents": docs,
		},
		"parameters": map[string]any{
			"return_documents": true,
			"top_n":            topN,
			"instruct":         "Given a web search query, retrieve relevant passages that answer the query.",
		},
	}

	var resp struct {
		Output struct {
			Results []struct {
				Index    int `json:"index"`
				Document any `json:"document"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := c.postJSON(ctx, c.nativeBaseURL+"/api/v1/services/rerank/text-rerank/text-rerank", body, &resp); err != nil {
		return nil, err
	}

	out := make([]string, 0, topN)
	for _, item := range resp.Output.Results {
		if text, ok := item.Document.(string); ok && text != "" {
			out = append(out, text)
			continue
		}
		if m, ok := item.Document.(map[string]any); ok {
			if text, ok := m["text"].(string); ok && text != "" {
				out = append(out, text)
				continue
			}
		}
		if item.Index >= 0 && item.Index < len(docs) {
			out = append(out, docs[item.Index])
		}
	}
	return out, nil
}

func (c *Client) postJSON(ctx context.Context, url string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dashscope %s: %s", resp.Status, string(raw))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}
