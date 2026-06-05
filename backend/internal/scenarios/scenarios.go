package scenarios

import (
	"fmt"

	"t2t/backend/internal/domain"
)

var all = []domain.Scenario{
	{
		ID:            "interview",
		Name:          "Interview",
		Description:   "Practice behavioral and technical interview answers with a realistic interviewer.",
		UserRole:      "Candidate",
		AssistantRole: "Interviewer",
		OpeningPrompt: "Welcome. I would like to learn about your background first. Could you tell me about a recent project you are proud of?",
		Tags:          []string{"career", "STAR", "follow-up"},
	},
	{
		ID:            "restaurant",
		Name:          "Restaurant Ordering",
		Description:   "Order food, ask about ingredients, handle recommendations, and pay politely.",
		UserRole:      "Customer",
		AssistantRole: "Server",
		OpeningPrompt: "Good evening. Welcome in. Would you like to start with something to drink while you look at the menu?",
		Tags:          []string{"daily", "polite requests", "food"},
	},
	{
		ID:            "meeting",
		Name:          "Business Meeting",
		Description:   "Discuss agenda items, clarify decisions, and respond to stakeholder questions.",
		UserRole:      "Team member",
		AssistantRole: "Meeting host",
		OpeningPrompt: "Thanks for joining. Let's begin with your update. What progress did your team make this week?",
		Tags:          []string{"work", "clarifying", "decisions"},
	},
	{
		ID:            "travel",
		Name:          "Travel Help",
		Description:   "Ask for directions, handle hotel check-in, and solve travel problems.",
		UserRole:      "Traveler",
		AssistantRole: "Local assistant",
		OpeningPrompt: "Hi there. You look like you might need help. Where are you trying to go today?",
		Tags:          []string{"travel", "directions", "problem solving"},
	},
	{
		ID:            "small-talk",
		Name:          "Small Talk",
		Description:   "Build confidence with friendly everyday conversations.",
		UserRole:      "Guest",
		AssistantRole: "Conversation partner",
		OpeningPrompt: "Hi, it's nice to meet you. How has your day been so far?",
		Tags:          []string{"daily", "rapport", "fluency"},
	},
}

func List() []domain.Scenario {
	return append([]domain.Scenario(nil), all...)
}

func Get(id string) (domain.Scenario, error) {
	for _, scenario := range all {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return domain.Scenario{}, fmt.Errorf("unknown scenario: %s", id)
}
