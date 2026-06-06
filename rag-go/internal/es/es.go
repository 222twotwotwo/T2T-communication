package es

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llmentor/rag-go/internal/config"
	"llmentor/rag-go/internal/document"
)

type DocumentChunk struct {
	ID       string         `json:"id"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Score    float64        `json:"score,omitempty"`
}

type Client struct {
	baseURL  string
	index    string
	username string
	password string
	http     *http.Client
}

func New(cfg config.ElasticsearchConfig) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.URL, "/"),
		index:    cfg.Index,
		username: cfg.Username,
		password: cfg.Password,
		http:     &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodHead, "/"+c.index, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("elasticsearch index check failed: %s", resp.Status)
	}
	req, err = c.newRequest(ctx, http.MethodPut, "/"+c.index, strings.NewReader(indexMappingJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create es index: %s %s", resp.Status, string(raw))
	}
	return nil
}

func (c *Client) BulkIndex(ctx context.Context, docs []document.Document) error {
	var chunks []DocumentChunk
	for _, doc := range docs {
		chunks = append(chunks, DocumentChunk{ID: doc.ID, Content: doc.Text, Metadata: doc.Metadata})
	}
	return c.BulkIndexChunks(ctx, chunks)
}

func (c *Client) BulkIndexChunks(ctx context.Context, chunks []DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, chunk := range chunks {
		if err := enc.Encode(map[string]any{"index": map[string]any{"_index": c.index, "_id": chunk.ID}}); err != nil {
			return err
		}
		if err := enc.Encode(chunk); err != nil {
			return err
		}
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/_bulk?refresh=true", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("es bulk index: %s %s", resp.Status, string(raw))
	}
	var parsed struct {
		Errors bool `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Errors {
		return errors.New("es bulk index completed with item errors")
	}
	return nil
}

func (c *Client) SearchByKeyword(ctx context.Context, keyword string, size int, category string) ([]DocumentChunk, error) {
	if size <= 0 {
		size = 5
	}
	matchQuery := map[string]any{
		"match": map[string]any{
			"content": map[string]any{"query": keyword},
		},
	}
	query := any(matchQuery)
	if category = strings.TrimSpace(category); category != "" {
		query = map[string]any{
			"bool": map[string]any{
				"must": []any{matchQuery},
				"filter": []any{
					map[string]any{"term": map[string]any{"metadata.category": category}},
				},
			},
		}
	}
	body := map[string]any{
		"query": query,
		"size":  size,
	}
	data, _ := json.Marshal(body)
	req, err := c.newRequest(ctx, http.MethodPost, "/"+c.index+"/_search", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("es search: %s %s", resp.Status, string(raw))
	}
	var parsed struct {
		Hits struct {
			Hits []struct {
				Score  float64       `json:"_score"`
				Source DocumentChunk `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]DocumentChunk, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		chunk := hit.Source
		chunk.Score = hit.Score
		out = append(out, chunk)
	}
	return out, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return req, nil
}

const indexMappingJSON = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0,
    "analysis": {
      "filter": {
        "my_stop_filter": {
          "type": "stop",
          "stopwords": "_chinese_"
        }
      },
      "analyzer": {
        "ik_max": {
          "type": "custom",
          "tokenizer": "ik_max_word",
          "filter": ["lowercase", "my_stop_filter"]
        },
        "ik_smart": {
          "type": "custom",
          "tokenizer": "ik_smart",
          "filter": ["lowercase", "my_stop_filter"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id": { "type": "keyword" },
      "content": {
        "type": "text",
        "analyzer": "ik_max",
        "search_analyzer": "ik_smart",
        "fields": {
          "smart": {
            "type": "text",
            "analyzer": "ik_smart",
            "search_analyzer": "ik_smart"
          }
        }
      },
      "metadata": {
        "type": "object",
        "properties": {
          "source": { "type": "keyword" },
          "category": { "type": "keyword" },
          "orderId": { "type": "keyword" }
        }
      }
    }
  }
}`
