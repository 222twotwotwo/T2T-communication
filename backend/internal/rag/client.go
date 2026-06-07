package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"t2t/backend/internal/config"
)

type Retriever interface {
	Search(ctx context.Context, request SearchRequest) ([]string, error)
}

type SearchRequest struct {
	Query           string
	Category        string
	TopK            int
	DashScopeAPIKey string
}

type IngestRequest struct {
	FilePath        string
	Category        string
	DashScopeAPIKey string
}

type IngestResult struct {
	Status      string `json:"status"`
	Category    string `json:"category"`
	RAGResponse string `json:"ragResponse"`
}

type DisabledRetriever struct{}

func (DisabledRetriever) Search(context.Context, SearchRequest) ([]string, error) {
	return nil, nil
}

type Client struct {
	cfg  config.RAGConfig
	http *http.Client
}

func NewRetriever(cfg config.RAGConfig) Retriever {
	return NewClient(cfg)
}

func NewClient(cfg config.RAGConfig) *Client {
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Search(ctx context.Context, request SearchRequest) ([]string, error) {
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return nil, nil
	}
	dashScopeAPIKey := strings.TrimSpace(request.DashScopeAPIKey)
	if !c.cfg.Enabled && dashScopeAPIKey == "" {
		return nil, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("rag baseURL is empty")
	}
	topK := request.TopK
	if topK <= 0 {
		topK = c.cfg.TopK
	}
	if topK <= 0 {
		topK = 5
	}

	endpoint, err := url.Parse(baseURL + "/rag/hybrid/searchFromHybrid")
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("keyword", query)
	q.Set("topK", strconv.Itoa(topK))
	q.Set("rerank", strconv.FormatBool(c.cfg.UseRerank))
	if category := strings.TrimSpace(request.Category); category != "" {
		q.Set("category", category)
	}
	endpoint.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if dashScopeAPIKey != "" {
		httpReq.Header.Set("X-T2T-DashScope-Key", dashScopeAPIKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rag search failed: %s", resp.Status)
	}
	var snippets []string
	if err := json.NewDecoder(resp.Body).Decode(&snippets); err != nil {
		return nil, err
	}
	return cleanSnippets(snippets, topK), nil
}

func (c *Client) Ingest(ctx context.Context, request IngestRequest) (IngestResult, error) {
	filePath := strings.TrimSpace(request.FilePath)
	if filePath == "" {
		return IngestResult{}, errors.New("file path is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if baseURL == "" {
		return IngestResult{}, errors.New("rag baseURL is empty")
	}

	endpoint, err := url.Parse(baseURL + "/rag/hybrid/write")
	if err != nil {
		return IngestResult{}, err
	}
	q := endpoint.Query()
	q.Set("filePath", filePath)
	if category := strings.TrimSpace(request.Category); category != "" {
		q.Set("category", category)
	}
	endpoint.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return IngestResult{}, err
	}
	if dashScopeAPIKey := strings.TrimSpace(request.DashScopeAPIKey); dashScopeAPIKey != "" {
		httpReq.Header.Set("X-T2T-DashScope-Key", dashScopeAPIKey)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return IngestResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return IngestResult{}, fmt.Errorf("rag ingest failed: %s", message)
	}
	responseText := strings.TrimSpace(string(body))
	if responseText == "" {
		responseText = "success"
	}
	return IngestResult{
		Status:      "success",
		Category:    strings.TrimSpace(request.Category),
		RAGResponse: responseText,
	}, nil
}

func cleanSnippets(snippets []string, limit int) []string {
	if limit <= 0 {
		limit = len(snippets)
	}
	out := make([]string, 0, min(len(snippets), limit))
	seen := map[string]bool{}
	for _, snippet := range snippets {
		snippet = strings.TrimSpace(snippet)
		if snippet == "" || seen[snippet] {
			continue
		}
		seen[snippet] = true
		out = append(out, snippet)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
