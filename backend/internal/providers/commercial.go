package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
)

type OpenAIProvider struct {
	cfg    config.OpenAIConfig
	client *http.Client
}

func NewOpenAIProvider(cfg config.OpenAIConfig) OpenAIProvider {
	return OpenAIProvider{cfg: cfg, client: &http.Client{Timeout: 25 * time.Second}}
}

func (p OpenAIProvider) Name() string {
	return "openai"
}

func (p OpenAIProvider) NextReply(ctx context.Context, request domain.ConversationContext) (domain.LLMReply, error) {
	prompt := buildConversationPrompt(request)
	text, err := p.chat(ctx, []chatMessage{
		{Role: "system", Content: "You are a warm, realistic English speaking partner. Continue the role-play naturally. Do not reveal corrections during the conversation."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return domain.LLMReply{}, err
	}
	return domain.LLMReply{Text: text, HiddenCorrections: []domain.Correction{}, Provider: p.Name()}, nil
}

func (p OpenAIProvider) GenerateReport(ctx context.Context, request domain.ReportContext) (domain.PracticeReport, error) {
	prompt := buildReportPrompt(request.Session)
	text, err := p.chat(ctx, []chatMessage{
		{Role: "system", Content: "You are an English speaking coach. Produce concise, kind, actionable feedback."},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return domain.PracticeReport{}, err
	}

	mock := NewMockProvider()
	report, _ := mock.GenerateReport(ctx, request)
	report.Summary = text
	return report, nil
}

func (p OpenAIProvider) chat(ctx context.Context, messages []chatMessage) (string, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return "", errors.New("openai api key is required for commercial llm provider")
	}
	body := map[string]any{
		"model":       p.cfg.Model,
		"messages":    messages,
		"temperature": 0.7,
	}
	payload, _ := json.Marshal(body)
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai chat failed: %s", resp.Status)
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	if len(decoded.Choices) == 0 {
		return "", errors.New("openai returned no choices")
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content), nil
}

type AnthropicProvider struct {
	cfg    config.AnthropicConfig
	client *http.Client
}

func NewAnthropicProvider(cfg config.AnthropicConfig) AnthropicProvider {
	return AnthropicProvider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
}

func (p AnthropicProvider) Name() string {
	return "anthropic"
}

func (p AnthropicProvider) NextReply(ctx context.Context, request domain.ConversationContext) (domain.LLMReply, error) {
	text, err := p.message(ctx, "You are a warm, realistic English speaking partner. Continue the role-play naturally. Do not reveal corrections during the conversation.", buildConversationPrompt(request))
	if err != nil {
		return domain.LLMReply{}, err
	}
	return domain.LLMReply{Text: text, HiddenCorrections: []domain.Correction{}, Provider: p.Name()}, nil
}

func (p AnthropicProvider) GenerateReport(ctx context.Context, request domain.ReportContext) (domain.PracticeReport, error) {
	text, err := p.message(ctx, "You are an English speaking coach. Produce concise, kind, actionable feedback.", buildReportPrompt(request.Session))
	if err != nil {
		return domain.PracticeReport{}, err
	}
	mock := NewMockProvider()
	report, _ := mock.GenerateReport(ctx, request)
	report.Summary = text
	return report, nil
}

func (p AnthropicProvider) message(ctx context.Context, system string, prompt string) (string, error) {
	if strings.TrimSpace(p.cfg.APIKey) == "" {
		return "", errors.New("anthropic api key is required for commercial llm provider")
	}
	body := map[string]any{
		"model":      p.cfg.Model,
		"max_tokens": 700,
		"system":     system,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	payload, _ := json.Marshal(body)
	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", p.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic message failed: %s", resp.Status)
	}
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", err
	}
	for _, block := range decoded.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", errors.New("anthropic returned no text content")
}

type AzureSpeechProvider struct {
	cfg config.AzureConfig
}

func NewAzureSpeechProvider(cfg config.AzureConfig) AzureSpeechProvider {
	return AzureSpeechProvider{cfg: cfg}
}

func (p AzureSpeechProvider) Name() string {
	return "azure"
}

func (p AzureSpeechProvider) Transcribe(_ context.Context, _ domain.ASRRequest) (domain.ASRResult, error) {
	if strings.TrimSpace(p.cfg.SpeechKey) == "" || strings.TrimSpace(p.cfg.SpeechRegion) == "" {
		return domain.ASRResult{}, errors.New("azure speech key and region are required for commercial asr provider")
	}
	return domain.ASRResult{}, errors.New("azure streaming asr adapter boundary is configured; wire the Speech SDK or websocket relay in deployment")
}

func (p AzureSpeechProvider) Evaluate(_ context.Context, _ domain.PronunciationRequest) (domain.PronunciationResult, error) {
	if strings.TrimSpace(p.cfg.SpeechKey) == "" || strings.TrimSpace(p.cfg.SpeechRegion) == "" {
		return domain.PronunciationResult{}, errors.New("azure speech key and region are required for pronunciation assessment")
	}
	return domain.PronunciationResult{}, errors.New("azure pronunciation assessment boundary is configured; wire the Speech SDK or REST relay in deployment")
}

type EdgeTTSProvider struct {
	cfg config.EdgeTTSConfig
}

func NewEdgeTTSProvider(cfg config.EdgeTTSConfig) EdgeTTSProvider {
	return EdgeTTSProvider{cfg: cfg}
}

func (p EdgeTTSProvider) Name() string {
	return "edge-tts"
}

func (p EdgeTTSProvider) Synthesize(ctx context.Context, request domain.TTSRequest) (domain.TTSResult, error) {
	text := strings.TrimSpace(request.Text)
	if text == "" {
		return domain.TTSResult{}, errors.New("tts text is required")
	}
	tmp, err := os.CreateTemp("", "t2t-tts-*.mp3")
	if err != nil {
		return domain.TTSResult{}, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	voice := request.Voice
	if voice == "" {
		voice = p.cfg.Voice
	}
	cmd := exec.CommandContext(ctx, p.cfg.Command, "--voice", voice, "--text", text, "--write-media", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return domain.TTSResult{}, fmt.Errorf("edge-tts failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.TTSResult{}, err
	}
	return domain.TTSResult{
		AudioBase64: base64.StdEncoding.EncodeToString(data),
		MimeType:    "audio/mpeg",
		Provider:    p.Name(),
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func buildConversationPrompt(request domain.ConversationContext) string {
	lines := []string{
		"Scenario: " + request.Session.Scenario.Name,
		"Assistant role: " + request.Session.Scenario.AssistantRole,
		"User role: " + request.Session.Scenario.UserRole,
		"Level: " + request.Session.Level,
		"Latest user sentence: " + request.UserText,
		"Reply in 1-2 natural spoken English sentences and ask one follow-up question when appropriate.",
	}
	return strings.Join(lines, "\n")
}

func buildReportPrompt(session domain.Session) string {
	lines := []string{
		"Create a concise English speaking practice report.",
		"Scenario: " + session.Scenario.Name,
		"Level: " + session.Level,
		"Conversation:",
	}
	for _, message := range session.Messages {
		lines = append(lines, message.Role+": "+message.Text)
	}
	lines = append(lines, "Return a friendly summary, top corrections, and a concrete 3-step practice plan.")
	return strings.Join(lines, "\n")
}
