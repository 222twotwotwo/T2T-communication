package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
)

func TestRuntimeLLMProviderUsesOpenAIKeyFromCredentials(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "runtime reply"}},
			},
		})
	}))
	defer server.Close()

	provider := NewRuntimeLLMProvider(config.ProvidersConfig{
		OpenAI: config.OpenAIConfig{
			BaseURL: server.URL,
			Model:   "test-model",
		},
	}, NewMockProvider())

	reply, err := provider.NextReply(context.Background(), domain.ConversationContext{
		Session: domain.Session{
			Scenario: domain.Scenario{Name: "Interview", AssistantRole: "Interviewer", UserRole: "Candidate"},
			Level:    "B1",
		},
		UserText: "Tell me about your project.",
		Credentials: domain.RuntimeCredentials{
			LLMProvider:  "openai",
			OpenAIAPIKey: "user-openai-key",
		},
	})
	if err != nil {
		t.Fatalf("next reply: %v", err)
	}
	if reply.Text != "runtime reply" {
		t.Fatalf("unexpected reply: %q", reply.Text)
	}
	if authorization != "Bearer user-openai-key" {
		t.Fatalf("expected user key authorization header, got %q", authorization)
	}
}

func TestRuntimeLLMProviderDoesNotFallbackWhenSelectedProviderHasNoKey(t *testing.T) {
	provider := NewRuntimeLLMProvider(config.ProvidersConfig{}, NewMockProvider())

	_, err := provider.NextReply(context.Background(), domain.ConversationContext{
		Credentials: domain.RuntimeCredentials{LLMProvider: "openai"},
	})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	if !strings.Contains(err.Error(), "openai api key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
