package providers

import (
	"context"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
)

type ASRProvider interface {
	Name() string
	Transcribe(ctx context.Context, request domain.ASRRequest) (domain.ASRResult, error)
}

type TTSProvider interface {
	Name() string
	Synthesize(ctx context.Context, request domain.TTSRequest) (domain.TTSResult, error)
}

type PronunciationProvider interface {
	Name() string
	Evaluate(ctx context.Context, request domain.PronunciationRequest) (domain.PronunciationResult, error)
}

type LLMProvider interface {
	Name() string
	NextReply(ctx context.Context, request domain.ConversationContext) (domain.LLMReply, error)
	GenerateReport(ctx context.Context, request domain.ReportContext) (domain.PracticeReport, error)
}

type Bundle struct {
	Mode          string
	ASR           ASRProvider
	TTS           TTSProvider
	Pronunciation PronunciationProvider
	LLM           LLMProvider
}

type Status struct {
	Mode            string            `json:"mode"`
	Providers       map[string]string `json:"providers"`
	CommercialReady bool              `json:"commercialReady"`
	Warnings        []string          `json:"warnings"`
}

func NewProviderBundle(cfg config.AppConfig) Bundle {
	mock := NewMockProvider()
	bundle := Bundle{
		Mode:          cfg.Providers.Mode,
		ASR:           mock,
		TTS:           mock,
		Pronunciation: mock,
		LLM:           mock,
	}

	if cfg.Providers.Mode == "commercial" {
		switch cfg.Providers.ASR {
		case "azure":
			bundle.ASR = NewAzureSpeechProvider(cfg.Providers.Azure)
		}
		switch cfg.Providers.Pronunciation {
		case "azure":
			bundle.Pronunciation = NewAzureSpeechProvider(cfg.Providers.Azure)
		}
		switch cfg.Providers.TTS {
		case "edge-tts":
			bundle.TTS = NewEdgeTTSProvider(cfg.Providers.EdgeTTS)
		}
		switch cfg.Providers.LLM {
		case "openai":
			bundle.LLM = NewOpenAIProvider(cfg.Providers.OpenAI)
		case "anthropic":
			bundle.LLM = NewAnthropicProvider(cfg.Providers.Anthropic)
		}
	}

	return bundle
}

func (b Bundle) Status() Status {
	warnings := []string{}
	commercialReady := true
	for capability, name := range map[string]string{
		"asr":           b.ASR.Name(),
		"tts":           b.TTS.Name(),
		"pronunciation": b.Pronunciation.Name(),
		"llm":           b.LLM.Name(),
	} {
		if name == "mock" && b.Mode == "commercial" {
			commercialReady = false
			warnings = append(warnings, capability+" is still using mock provider")
		}
	}

	return Status{
		Mode: b.Mode,
		Providers: map[string]string{
			"asr":           b.ASR.Name(),
			"tts":           b.TTS.Name(),
			"pronunciation": b.Pronunciation.Name(),
			"llm":           b.LLM.Name(),
		},
		CommercialReady: commercialReady,
		Warnings:        warnings,
	}
}
