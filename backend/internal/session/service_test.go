package session

import (
	"context"
	"testing"

	"t2t/backend/internal/config"
	"t2t/backend/internal/domain"
	"t2t/backend/internal/providers"
	"t2t/backend/internal/rag"
)

func TestSessionCollectsIssuesSilentlyAndReports(t *testing.T) {
	bundle := providers.NewProviderBundle(config.AppConfig{
		Providers: config.ProvidersConfig{
			Mode:          "mock",
			ASR:           "mock",
			TTS:           "mock",
			Pronunciation: "mock",
			LLM:           "mock",
		},
	})
	service := NewService(bundle)

	created, err := service.Create(context.Background(), CreateRequest{ScenarioID: "interview", Level: "B1"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	turn, err := service.AddTurn(context.Background(), created.ID, TurnRequest{Text: "I has worked in team"})
	if err != nil {
		t.Fatalf("add turn: %v", err)
	}
	if turn.Signal.CollectedIssues == 0 {
		t.Fatalf("expected collected issues, got zero")
	}
	for _, message := range turn.Session.Messages {
		if message.Role == "assistant" && message.Text == "" {
			t.Fatalf("assistant reply should not be empty")
		}
	}

	report, err := service.Finish(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("finish session: %v", err)
	}
	if len(report.Corrections) == 0 {
		t.Fatalf("expected corrections in report")
	}
	if report.OverallScore.Grammar <= 0 {
		t.Fatalf("expected grammar score")
	}
}

func TestSessionInjectsRAGKnowledgeIntoLLMContext(t *testing.T) {
	mock := providers.NewMockProvider()
	llm := &capturingLLM{}
	retriever := &fakeRetriever{snippets: []string{"Use STAR: situation, task, action, and result."}}
	service := NewService(providers.Bundle{
		Mode:          "mock",
		ASR:           mock,
		TTS:           mock,
		Pronunciation: mock,
		LLM:           llm,
	}, retriever)

	created, err := service.Create(context.Background(), CreateRequest{ScenarioID: "interview", Level: "B1"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := service.AddTurn(context.Background(), created.ID, TurnRequest{Text: "I led a migration project last quarter."}); err != nil {
		t.Fatalf("add turn: %v", err)
	}
	if retriever.last.Category != "interview" {
		t.Fatalf("expected interview category, got %q", retriever.last.Category)
	}
	if len(llm.last.KnowledgeSnippets) != 1 {
		t.Fatalf("expected one knowledge snippet, got %d", len(llm.last.KnowledgeSnippets))
	}
	if llm.last.KnowledgeSnippets[0] != retriever.snippets[0] {
		t.Fatalf("unexpected knowledge snippet: %q", llm.last.KnowledgeSnippets[0])
	}
}

type fakeRetriever struct {
	snippets []string
	last     rag.SearchRequest
}

func (f *fakeRetriever) Search(_ context.Context, request rag.SearchRequest) ([]string, error) {
	f.last = request
	return f.snippets, nil
}

type capturingLLM struct {
	last domain.ConversationContext
}

func (c *capturingLLM) Name() string {
	return "capturing"
}

func (c *capturingLLM) NextReply(_ context.Context, request domain.ConversationContext) (domain.LLMReply, error) {
	c.last = request
	return domain.LLMReply{Text: "Thanks. What result did you achieve?", Provider: c.Name()}, nil
}

func (c *capturingLLM) GenerateReport(context.Context, domain.ReportContext) (domain.PracticeReport, error) {
	return domain.PracticeReport{}, nil
}
