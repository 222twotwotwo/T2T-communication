package providers

import (
	"context"
	"strings"
	"testing"

	"t2t/backend/internal/domain"
)

func TestMockNextReplyDoesNotShortCircuitShortUserText(t *testing.T) {
	reply, err := NewMockProvider().NextReply(context.Background(), domain.ConversationContext{
		Session: domain.Session{
			Scenario: domain.Scenario{ID: "interview"},
		},
		UserText:  "yes",
		TurnIndex: 1,
	})
	if err != nil {
		t.Fatalf("next reply: %v", err)
	}
	if reply.Text == "" {
		t.Fatalf("expected reply text")
	}
	if strings.Contains(reply.Text, "Could you add one specific detail") {
		t.Fatalf("short user text should not return fixed short-follow-up reply: %q", reply.Text)
	}
}
