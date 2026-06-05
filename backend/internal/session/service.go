package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"t2t/backend/internal/domain"
	"t2t/backend/internal/providers"
	"t2t/backend/internal/scenarios"
)

type Service struct {
	providers providers.Bundle
	mu        sync.RWMutex
	sessions  map[string]domain.Session
}

type CreateRequest struct {
	ScenarioID string `json:"scenarioId"`
	Level      string `json:"level"`
	Voice      string `json:"voice"`
}

type TurnRequest struct {
	Text        string `json:"text"`
	AudioBase64 string `json:"audioBase64"`
	MimeType    string `json:"mimeType"`
}

type TurnResponse struct {
	Session          domain.SessionSnapshot `json:"session"`
	UserMessage      domain.Message         `json:"userMessage"`
	AssistantMessage domain.Message         `json:"assistantMessage"`
	Signal           domain.TurnSignal      `json:"signal"`
	TranscriptSource string                 `json:"transcriptSource"`
}

func NewService(bundle providers.Bundle) *Service {
	return &Service{
		providers: bundle,
		sessions:  map[string]domain.Session{},
	}
}

func (s *Service) ProviderStatus() providers.Status {
	return s.providers.Status()
}

func (s *Service) Create(ctx context.Context, request CreateRequest) (domain.SessionSnapshot, error) {
	scenarioID := strings.TrimSpace(request.ScenarioID)
	if scenarioID == "" {
		scenarioID = "interview"
	}
	scenario, err := scenarios.Get(scenarioID)
	if err != nil {
		return domain.SessionSnapshot{}, err
	}

	level := strings.TrimSpace(request.Level)
	if level == "" {
		level = "B1"
	}

	now := time.Now().UTC()
	session := domain.Session{
		ID:             newID("sess"),
		Scenario:       scenario,
		Level:          level,
		Voice:          strings.TrimSpace(request.Voice),
		StartedAt:      now,
		LastActivityAt: now,
		Messages: []domain.Message{
			{
				ID:        newID("msg"),
				Role:      "assistant",
				Text:      scenario.OpeningPrompt,
				CreatedAt: now,
			},
		},
	}

	if session.Voice == "" {
		session.Voice = "en-US-JennyNeural"
	}

	if session.Scenario.OpeningPrompt == "" {
		reply, err := s.providers.LLM.NextReply(ctx, domain.ConversationContext{
			Session:   session,
			UserText:  "Start the scenario.",
			TurnIndex: 0,
			StartedAt: now,
		})
		if err != nil {
			return domain.SessionSnapshot{}, err
		}
		session.Messages[0].Text = reply.Text
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return snapshot(session), nil
}

func (s *Service) Get(id string) (domain.SessionSnapshot, error) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return domain.SessionSnapshot{}, errors.New("session not found")
	}
	return snapshot(session), nil
}

func (s *Service) AddTurn(ctx context.Context, id string, request TurnRequest) (TurnResponse, error) {
	startedAt := time.Now().UTC()

	s.mu.RLock()
	current, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return TurnResponse{}, errors.New("session not found")
	}
	if current.EndedAt != nil {
		return TurnResponse{}, errors.New("session already ended")
	}

	userText := strings.TrimSpace(request.Text)
	transcriptSource := "text"
	if userText == "" {
		asr, err := s.providers.ASR.Transcribe(ctx, domain.ASRRequest{
			AudioBase64: request.AudioBase64,
			MimeType:    request.MimeType,
			ScenarioID:  current.Scenario.ID,
		})
		if err != nil {
			return TurnResponse{}, err
		}
		userText = strings.TrimSpace(asr.Transcript)
		transcriptSource = s.providers.ASR.Name()
	}
	if userText == "" {
		return TurnResponse{}, errors.New("text or audio is required")
	}

	turnIndex := current.TranscriptCount + 1
	pronunciation, err := s.providers.Pronunciation.Evaluate(ctx, domain.PronunciationRequest{
		Text:        userText,
		AudioBase64: request.AudioBase64,
		MimeType:    request.MimeType,
		ScenarioID:  current.Scenario.ID,
		TurnIndex:   turnIndex,
	})
	if err != nil {
		return TurnResponse{}, err
	}

	userMessage := domain.Message{
		ID:        newID("msg"),
		Role:      "user",
		Text:      userText,
		CreatedAt: time.Now().UTC(),
	}

	contextSession := current
	contextSession.Messages = append(contextSession.Messages, userMessage)
	llmReply, err := s.providers.LLM.NextReply(ctx, domain.ConversationContext{
		Session:   contextSession,
		UserText:  userText,
		TurnIndex: turnIndex,
		StartedAt: startedAt,
	})
	if err != nil {
		return TurnResponse{}, err
	}

	tts, err := s.providers.TTS.Synthesize(ctx, domain.TTSRequest{
		Text:  llmReply.Text,
		Voice: current.Voice,
	})
	if err != nil && s.providers.TTS.Name() != "mock" {
		return TurnResponse{}, err
	}

	assistantMessage := domain.Message{
		ID:        newID("msg"),
		Role:      "assistant",
		Text:      llmReply.Text,
		CreatedAt: time.Now().UTC(),
	}
	if tts.AudioBase64 != "" {
		assistantMessage.AudioURL = "data:" + tts.MimeType + ";base64," + tts.AudioBase64
	}

	latencyMs := time.Since(startedAt).Milliseconds()
	signal := domain.TurnSignal{
		TurnIndex:       turnIndex,
		Score:           pronunciation.Score,
		WordsPerMinute:  estimateWordsPerMinute(userText),
		LatencyMs:       latencyMs,
		CollectedIssues: len(pronunciation.Findings) + len(llmReply.HiddenCorrections),
	}

	s.mu.Lock()
	updated := s.sessions[id]
	updated.Messages = append(updated.Messages, userMessage, assistantMessage)
	updated.Signals = append(updated.Signals, signal)
	updated.HiddenFindings = append(updated.HiddenFindings, pronunciation.Findings...)
	updated.HiddenFindings = append(updated.HiddenFindings, llmReply.HiddenCorrections...)
	updated.LastActivityAt = time.Now().UTC()
	updated.TranscriptCount++
	s.sessions[id] = updated
	s.mu.Unlock()

	return TurnResponse{
		Session:          snapshot(updated),
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Signal:           signal,
		TranscriptSource: transcriptSource,
	}, nil
}

func (s *Service) Finish(ctx context.Context, id string) (domain.PracticeReport, error) {
	s.mu.Lock()
	current, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return domain.PracticeReport{}, errors.New("session not found")
	}
	if current.EndedAt == nil {
		now := time.Now().UTC()
		current.EndedAt = &now
		current.LastActivityAt = now
		s.sessions[id] = current
	}
	s.mu.Unlock()

	return s.providers.LLM.GenerateReport(ctx, domain.ReportContext{Session: current})
}

func snapshot(session domain.Session) domain.SessionSnapshot {
	messages := append([]domain.Message{}, session.Messages...)
	signals := append([]domain.TurnSignal{}, session.Signals...)
	return domain.SessionSnapshot{
		ID:             session.ID,
		Scenario:       session.Scenario,
		Level:          session.Level,
		Voice:          session.Voice,
		Messages:       messages,
		Signals:        signals,
		StartedAt:      session.StartedAt,
		EndedAt:        session.EndedAt,
		CollectedCount: len(session.HiddenFindings),
	}
}

func estimateWordsPerMinute(text string) int {
	words := len(strings.Fields(text))
	if words == 0 {
		return 0
	}
	return 110 + min(words*3, 45)
}

func newID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return prefix + "-" + hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return prefix + "-" + hex.EncodeToString(bytes)
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
