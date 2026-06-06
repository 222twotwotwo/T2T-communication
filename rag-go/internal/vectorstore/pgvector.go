package vectorstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"llmentor/rag-go/internal/config"
	"llmentor/rag-go/internal/document"
)

type Store struct {
	pool       *pgxpool.Pool
	table      string
	dimensions int
}

var tableNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func New(ctx context.Context, cfg config.PostgresConfig) (*Store, error) {
	if cfg.DSN == "" {
		return nil, errors.New("postgres dsn is empty")
	}
	if !tableNameRE.MatchString(cfg.Table) {
		return nil, fmt.Errorf("invalid postgres table name %q", cfg.Table)
	}
	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, err
	}
	store := &Store{pool: pool, table: cfg.Table, dimensions: cfg.Dimensions}
	if cfg.InitializeSchema {
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	if s.dimensions <= 0 {
		s.dimensions = 768
	}
	sql := fmt.Sprintf(`
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE IF NOT EXISTS %s (
  id uuid PRIMARY KEY,
  content text,
  metadata json,
  embedding vector(%d)
);
CREATE INDEX IF NOT EXISTS %s_embedding_hnsw_idx ON %s USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS %s_metadata_category_idx ON %s ((metadata->>'category'));
`, s.table, s.dimensions, s.table, s.table, s.table, s.table)
	_, err := s.pool.Exec(ctx, sql)
	return err
}

func (s *Store) Add(ctx context.Context, docs []document.Document, embeddings [][]float64) error {
	if len(docs) != len(embeddings) {
		return fmt.Errorf("document/embedding count mismatch: %d/%d", len(docs), len(embeddings))
	}
	batchSize := 9
	for start := 0; start < len(docs); start += batchSize {
		end := start + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		for i := start; i < end; i++ {
			meta, err := json.Marshal(docs[i].Metadata)
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
			_, err = tx.Exec(ctx,
				fmt.Sprintf("INSERT INTO %s (id, content, metadata, embedding) VALUES ($1::uuid, $2, $3::json, $4::vector) ON CONFLICT (id) DO UPDATE SET content = EXCLUDED.content, metadata = EXCLUDED.metadata, embedding = EXCLUDED.embedding", s.table),
				docs[i].ID, docs[i].Text, string(meta), vectorLiteral(embeddings[i]),
			)
			if err != nil {
				_ = tx.Rollback(ctx)
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Search(ctx context.Context, queryEmbedding []float64, threshold float64, topK int, category string) ([]document.Document, error) {
	if topK <= 0 {
		topK = 5
	}
	if threshold <= 0 {
		threshold = 0.5
	}
	category = strings.TrimSpace(category)
	query := fmt.Sprintf(`
SELECT id::text, content, COALESCE(metadata::text, '{}'), 1 - (embedding <=> $1::vector) AS similarity
FROM %s
WHERE 1 - (embedding <=> $1::vector) >= $2
ORDER BY embedding <=> $1::vector
LIMIT $3`, s.table)
	args := []any{vectorLiteral(queryEmbedding), threshold, topK}
	if category != "" {
		query = fmt.Sprintf(`
SELECT id::text, content, COALESCE(metadata::text, '{}'), 1 - (embedding <=> $1::vector) AS similarity
FROM %s
WHERE 1 - (embedding <=> $1::vector) >= $2
  AND metadata->>'category' = $4
ORDER BY embedding <=> $1::vector
LIMIT $3`, s.table)
		args = append(args, category)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []document.Document
	for rows.Next() {
		var doc document.Document
		var metadataRaw string
		if err := rows.Scan(&doc.ID, &doc.Text, &metadataRaw, &doc.Score); err != nil {
			return nil, err
		}
		doc.Metadata = map[string]any{}
		_ = json.Unmarshal([]byte(metadataRaw), &doc.Metadata)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func vectorLiteral(values []float64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}
