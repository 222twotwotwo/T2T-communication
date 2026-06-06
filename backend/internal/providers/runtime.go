package providers

import (
	"context"
	"errors"
	"fmt"
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
	provider, ok, err := p.providerFor(request.Credentials)
	if err != nil {
		return domain.LLMReply{}, err
	}
	if ok {
		return provider.NextReply(ctx, request)
	}
	return p.fallback.NextReply(ctx, request)
}

func (p RuntimeLLMProvider) GenerateReport(ctx context.Context, request domain.ReportContext) (domain.PracticeReport, error) {
	provider, ok, err := p.providerFor(request.Credentials)
	if err != nil {
		return domain.PracticeReport{}, err
	}
	if ok {
		return provider.GenerateReport(ctx, request)
	}
	return p.fallback.GenerateReport(ctx, request)
}

func (p RuntimeLLMProvider) providerFor(credentials domain.RuntimeCredentials) (LLMProvider, bool, error) {
	provider := strings.ToLower(strings.TrimSpace(credentials.LLMProvider))
	openAIKey := strings.TrimSpace(credentials.OpenAIAPIKey)
	anthropicKey := strings.TrimSpace(credentials.AnthropicAPIKey)
	explicitProvider := provider != "" && provider != "auto"
	if provider == "" || provider == "auto" {
		switch {
		case openAIKey != "":
			provider = "openai"
		case anthropicKey != "":
			provider = "anthropic"
		default:
			return nil, false, nil
		}
	}

	switch provider {
	case "openai":
		if openAIKey == "" {
			if explicitProvider {
				return nil, false, errors.New("openai api key is required for selected llm provider")
			}
			return nil, false, nil
		}
		cfg := p.cfg.OpenAI
		cfg.APIKey = openAIKey
		return NewOpenAIProvider(cfg), true, nil
	case "anthropic":
		if anthropicKey == "" {
			if explicitProvider {
				return nil, false, errors.New("anthropic api key is required for selected llm provider")
			}
			return nil, false, nil
		}
		cfg := p.cfg.Anthropic
		cfg.APIKey = anthropicKey
		return NewAnthropicProvider(cfg), true, nil
	case "mock":
		return nil, false, errors.New("mock llm provider is disabled; select openai or anthropic")
	default:
		if explicitProvider {
			return nil, false, fmt.Errorf("unsupported llm provider %q", provider)
		}
		return nil, false, nil
	}
}
