package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	DashScope     DashScopeConfig     `yaml:"dashscope"`
	Postgres      PostgresConfig      `yaml:"postgres"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	MinIO         MinIOConfig         `yaml:"minio"`
	Reader        ReaderConfig        `yaml:"reader"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type DashScopeConfig struct {
	APIKey              string `yaml:"api_key"`
	CompatibleBaseURL   string `yaml:"compatible_base_url"`
	NativeBaseURL       string `yaml:"native_base_url"`
	EmbeddingModel      string `yaml:"embedding_model"`
	EmbeddingDimensions int    `yaml:"embedding_dimensions"`
	ChatModel           string `yaml:"chat_model"`
	RerankModel         string `yaml:"rerank_model"`
}

type PostgresConfig struct {
	DSN              string `yaml:"dsn"`
	Table            string `yaml:"table"`
	Dimensions       int    `yaml:"dimensions"`
	InitializeSchema bool   `yaml:"initialize_schema"`
}

type ElasticsearchConfig struct {
	URL                string `yaml:"url"`
	Index              string `yaml:"index"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type MinIOConfig struct {
	Endpoint   string `yaml:"endpoint"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
	BucketName string `yaml:"bucket_name"`
	PublicRead bool   `yaml:"public_read"`
}

type ReaderConfig struct {
	TikaURL string `yaml:"tika_url"`
}

func Load(path string) (*Config, error) {
	cfg := defaults()
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	applyEnv(&cfg)
	return &cfg, nil
}

func defaults() Config {
	return Config{
		Server: ServerConfig{Port: 8001},
		DashScope: DashScopeConfig{
			CompatibleBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
			NativeBaseURL:       "https://dashscope.aliyuncs.com",
			EmbeddingModel:      "text-embedding-v4",
			EmbeddingDimensions: 768,
			ChatModel:           "qwen-plus",
			RerankModel:         "gte-rerank-v2",
		},
		Postgres: PostgresConfig{
			DSN:        "postgres://pgvector:pgvector@localhost:5433/rag_test?sslmode=disable",
			Table:      "vector_st",
			Dimensions: 768,
		},
		Elasticsearch: ElasticsearchConfig{
			URL:   "http://localhost:9200",
			Index: "rag_docs",
		},
		MinIO: MinIOConfig{
			Endpoint:   "http://localhost:9000",
			AccessKey:  "minioadmin",
			SecretKey:  "minioadmin",
			BucketName: "rag-test",
			PublicRead: true,
		},
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("RAG_GO_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	setString("DASHSCOPE_API_KEY", &cfg.DashScope.APIKey)
	setString("DASHSCOPE_COMPATIBLE_BASE_URL", &cfg.DashScope.CompatibleBaseURL)
	setString("DASHSCOPE_NATIVE_BASE_URL", &cfg.DashScope.NativeBaseURL)
	setString("DASHSCOPE_CHAT_MODEL", &cfg.DashScope.ChatModel)
	setString("DASHSCOPE_EMBEDDING_MODEL", &cfg.DashScope.EmbeddingModel)
	setString("DASHSCOPE_RERANK_MODEL", &cfg.DashScope.RerankModel)
	setString("PG_DSN", &cfg.Postgres.DSN)
	setString("PG_TABLE", &cfg.Postgres.Table)
	setString("ES_URL", &cfg.Elasticsearch.URL)
	setString("ES_INDEX", &cfg.Elasticsearch.Index)
	setString("ES_USERNAME", &cfg.Elasticsearch.Username)
	setString("ES_PASSWORD", &cfg.Elasticsearch.Password)
	setString("MINIO_ENDPOINT", &cfg.MinIO.Endpoint)
	setString("MINIO_ACCESS_KEY", &cfg.MinIO.AccessKey)
	setString("MINIO_SECRET_KEY", &cfg.MinIO.SecretKey)
	setString("MINIO_BUCKET", &cfg.MinIO.BucketName)
	setString("TIKA_URL", &cfg.Reader.TikaURL)
}

func setString(key string, target *string) {
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}
