package providers

import (
	"context"
	"strings"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
)

type RuntimeLLMProvider struct {
	cfg      config.ProvidersConfig
	fallback LLMProvider
}

func NewRuntimeLLMProvider(cfg config.ProvidersConfig, fallback LLMProvider) RuntimeLLMProvider {
	return RuntimeLLMProvider{cfg: cfg, fallback: fallback}
}

func (p RuntimeLLMProvider) Name() string {
	if p.fallback == nil {
		return "mock"
	}
	return p.fallback.Name()
}

func (p RuntimeLLMProvider) NextReply(ctx context.Context, request domain.ConversationContext) (domain.LLMReply, error) {
	if provider, ok := p.providerFor(request.Credentials); ok {
		return provider.NextReply(ctx, request)
	}
	return p.fallback.NextReply(ctx, request)
}

func (p RuntimeLLMProvider) GenerateReport(ctx context.Context, request domain.ReportContext) (domain.PracticeReport, error) {
	if provider, ok := p.providerFor(request.Credentials); ok {
		return provider.GenerateReport(ctx, request)
	}
	return p.fallback.GenerateReport(ctx, request)
}

func (p RuntimeLLMProvider) providerFor(credentials domain.RuntimeCredentials) (LLMProvider, bool) {
	provider := strings.ToLower(strings.TrimSpace(credentials.LLMProvider))
	openAIKey := strings.TrimSpace(credentials.OpenAIAPIKey)
	anthropicKey := strings.TrimSpace(credentials.AnthropicAPIKey)
	if provider == "" || provider == "auto" {
		switch {
		case openAIKey != "":
			provider = "openai"
		case anthropicKey != "":
			provider = "anthropic"
		default:
			return nil, false
		}
	}

	switch provider {
	case "openai":
		if openAIKey == "" {
			return nil, false
		}
		cfg := p.cfg.OpenAI
		cfg.APIKey = openAIKey
		return NewOpenAIProvider(cfg), true
	case "anthropic":
		if anthropicKey == "" {
			return nil, false
		}
		cfg := p.cfg.Anthropic
		cfg.APIKey = anthropicKey
		return NewAnthropicProvider(cfg), true
	default:
		return nil, false
	}
}
