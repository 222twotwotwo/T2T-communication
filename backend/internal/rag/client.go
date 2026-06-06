package rag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Query    string
	Category string
	TopK     int
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
	if !cfg.Enabled {
		return DisabledRetriever{}
	}
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
