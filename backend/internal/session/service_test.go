package session

import (
	"context"
	"testing"

	"t2t/backend/internal/config"
	"t2t/backend/internal/providers"
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
