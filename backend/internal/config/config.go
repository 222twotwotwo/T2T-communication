package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AppConfig struct {
	Server    ServerConfig    `json:"server"`
	Providers ProvidersConfig `json:"providers"`
	RAG       RAGConfig       `json:"rag"`
}

type ServerConfig struct {
	Port        string   `json:"port"`
	CorsOrigins []string `json:"corsOrigins"`
}

type ProvidersConfig struct {
	Mode          string          `json:"mode"`
	ASR           string          `json:"asr"`
	TTS           string          `json:"tts"`
	Pronunciation string          `json:"pronunciation"`
	LLM           string          `json:"llm"`
	Azure         AzureConfig     `json:"azure"`
	OpenAI        OpenAIConfig    `json:"openai"`
	Anthropic     AnthropicConfig `json:"anthropic"`
	EdgeTTS       EdgeTTSConfig   `json:"edgeTTS"`
}

type AzureConfig struct {
	SpeechKey    string `json:"speechKey"`
	SpeechRegion string `json:"speechRegion"`
	Language     string `json:"language"`
}

type OpenAIConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
}

type AnthropicConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
}

type EdgeTTSConfig struct {
	Voice   string `json:"voice"`
	Command string `json:"command"`
}

type RAGConfig struct {
	Enabled   bool   `json:"enabled"`
	BaseURL   string `json:"baseURL"`
	TopK      int    `json:"topK"`
	TimeoutMs int    `json:"timeoutMs"`
	UseRerank bool   `json:"useRerank"`
}

func Load() (AppConfig, []string) {
	cfg := defaults()
	warnings := []string{}

	path := strings.TrimSpace(os.Getenv("T2T_CONFIG_FILE"))
	if path == "" {
		path = firstExistingPath([]string{
			filepath.Join("config", "app.mock.json"),
			filepath.Join("..", "config", "app.mock.json"),
			filepath.Join("..", "..", "config", "app.mock.json"),
		})
	}

	if path != "" {
		if err := loadJSON(path, &cfg); err != nil {
			warnings = append(warnings, fmt.Sprintf("could not load %s: %v; using defaults", path, err))
		}
	} else {
		warnings = append(warnings, "no config file found; using built-in defaults")
	}

	applyEnvOverrides(&cfg)
	normalize(&cfg)
	return cfg, warnings
}

func defaults() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Port:        "8080",
			CorsOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		},
		Providers: ProvidersConfig{
			Mode:          "mock",
			ASR:           "mock",
			TTS:           "mock",
			Pronunciation: "mock",
			LLM:           "mock",
			Azure: AzureConfig{
				Language: "en-US",
			},
			OpenAI: OpenAIConfig{
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4o-mini",
			},
			Anthropic: AnthropicConfig{
				BaseURL: "https://api.anthropic.com",
				Model:   "claude-3-5-sonnet-latest",
			},
			EdgeTTS: EdgeTTSConfig{
				Voice:   "en-US-JennyNeural",
				Command: "edge-tts",
			},
		},
		RAG: RAGConfig{
			Enabled:   false,
			BaseURL:   "http://localhost:8001",
			TopK:      5,
			TimeoutMs: 1500,
			UseRerank: false,
		},
	}
}

func loadJSON(path string, cfg *AppConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, cfg)
}

func firstExistingPath(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func applyEnvOverrides(cfg *AppConfig) {
	if value := strings.TrimSpace(os.Getenv("T2T_PORT")); value != "" {
		cfg.Server.Port = value
	}
	if value := strings.TrimSpace(os.Getenv("T2T_PROVIDER_MODE")); value != "" {
		cfg.Providers.Mode = value
	}
	if value := strings.TrimSpace(os.Getenv("T2T_LLM_PROVIDER")); value != "" {
		cfg.Providers.LLM = value
	}
	if value := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); value != "" {
		cfg.Providers.OpenAI.APIKey = value
	}
	if value := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); value != "" {
		cfg.Providers.Anthropic.APIKey = value
	}
	if value := strings.TrimSpace(os.Getenv("AZURE_SPEECH_KEY")); value != "" {
		cfg.Providers.Azure.SpeechKey = value
	}
	if value := strings.TrimSpace(os.Getenv("AZURE_SPEECH_REGION")); value != "" {
		cfg.Providers.Azure.SpeechRegion = value
	}
	if value := strings.TrimSpace(os.Getenv("T2T_RAG_ENABLED")); value != "" {
		cfg.RAG.Enabled = parseBool(value)
	}
	if value := strings.TrimSpace(os.Getenv("T2T_RAG_BASE_URL")); value != "" {
		cfg.RAG.BaseURL = value
	}
	if value := strings.TrimSpace(os.Getenv("T2T_RAG_TOP_K")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.RAG.TopK = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv("T2T_RAG_TIMEOUT_MS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.RAG.TimeoutMs = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv("T2T_RAG_USE_RERANK")); value != "" {
		cfg.RAG.UseRerank = parseBool(value)
	}
}

func normalize(cfg *AppConfig) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	cfg.Providers.Mode = lowerDefault(cfg.Providers.Mode, "mock")
	cfg.Providers.ASR = lowerDefault(cfg.Providers.ASR, "mock")
	cfg.Providers.TTS = lowerDefault(cfg.Providers.TTS, "mock")
	cfg.Providers.Pronunciation = lowerDefault(cfg.Providers.Pronunciation, "mock")
	cfg.Providers.LLM = lowerDefault(cfg.Providers.LLM, "mock")
	if cfg.Providers.Azure.Language == "" {
		cfg.Providers.Azure.Language = "en-US"
	}
	if cfg.Providers.OpenAI.BaseURL == "" {
		cfg.Providers.OpenAI.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Providers.OpenAI.Model == "" {
		cfg.Providers.OpenAI.Model = "gpt-4o-mini"
	}
	if cfg.Providers.Anthropic.BaseURL == "" {
		cfg.Providers.Anthropic.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Providers.Anthropic.Model == "" {
		cfg.Providers.Anthropic.Model = "claude-3-5-sonnet-latest"
	}
	if cfg.Providers.EdgeTTS.Command == "" {
		cfg.Providers.EdgeTTS.Command = "edge-tts"
	}
	if cfg.Providers.EdgeTTS.Voice == "" {
		cfg.Providers.EdgeTTS.Voice = "en-US-JennyNeural"
	}
	cfg.RAG.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.RAG.BaseURL), "/")
	if cfg.RAG.BaseURL == "" {
		cfg.RAG.BaseURL = "http://localhost:8001"
	}
	if cfg.RAG.TopK <= 0 {
		cfg.RAG.TopK = 5
	}
	if cfg.RAG.TimeoutMs <= 0 {
		cfg.RAG.TimeoutMs = 1500
	}
}

func lowerDefault(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}
