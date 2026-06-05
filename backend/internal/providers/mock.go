package providers

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"t2t/backend/internal/domain"
)

type MockProvider struct{}

func NewMockProvider() MockProvider {
	return MockProvider{}
}

func (MockProvider) Name() string {
	return "mock"
}

func (MockProvider) Transcribe(_ context.Context, request domain.ASRRequest) (domain.ASRResult, error) {
	transcript := strings.TrimSpace(request.HintText)
	if transcript == "" {
		transcript = fallbackTranscript(request.ScenarioID)
	}
	return domain.ASRResult{
		Transcript: transcript,
		Confidence: 0.91,
		Provider:   "mock",
	}, nil
}

func (MockProvider) Synthesize(_ context.Context, _ domain.TTSRequest) (domain.TTSResult, error) {
	return domain.TTSResult{
		AudioBase64: "",
		MimeType:    "audio/mpeg",
		Provider:    "mock",
	}, nil
}

func (MockProvider) Evaluate(_ context.Context, request domain.PronunciationRequest) (domain.PronunciationResult, error) {
	text := strings.TrimSpace(request.Text)
	words := countWords(text)
	lengthPenalty := 0
	if words < 5 {
		lengthPenalty = 10
	}

	score := domain.ScoreCard{
		Pronunciation: clamp(86-lengthPenalty-len(findPronunciationIssues(text, request.TurnIndex))*4, 55, 98),
		Fluency:       clamp(82-lengthPenalty+min(words, 18)/3, 55, 96),
		Grammar:       clamp(88-len(findGrammarIssues(text, request.TurnIndex))*7, 55, 98),
		Vocabulary:    clamp(78+min(uniqueWordCount(text), 22)/2, 55, 96),
		Interaction:   clamp(80+min(words, 16)/2, 55, 96),
	}

	findings := append(findPronunciationIssues(text, request.TurnIndex), findGrammarIssues(text, request.TurnIndex)...)
	return domain.PronunciationResult{
		Score:    score,
		Findings: findings,
		Provider: "mock",
	}, nil
}

func (MockProvider) NextReply(_ context.Context, request domain.ConversationContext) (domain.LLMReply, error) {
	userText := strings.TrimSpace(request.UserText)
	scenarioID := request.Session.Scenario.ID
	turn := request.TurnIndex
	reply := nextScenarioReply(scenarioID, userText, turn)
	return domain.LLMReply{
		Text:              reply,
		HiddenCorrections: findExpressionIssues(userText, turn),
		Provider:          "mock",
	}, nil
}

func (MockProvider) GenerateReport(_ context.Context, request domain.ReportContext) (domain.PracticeReport, error) {
	session := request.Session
	overall := averageScores(session.Signals)
	totalIssues := len(session.HiddenFindings)
	cefr := estimateCEFR(overall)

	summary := fmt.Sprintf("You completed %d speaking turns in the %s scenario. Your answers stayed on topic and the next step is to make your sentences a little fuller and smoother.", session.TranscriptCount, session.Scenario.Name)
	if totalIssues == 0 {
		summary = fmt.Sprintf("You completed %d speaking turns in the %s scenario with clear, natural responses. Keep practicing longer turns to build automatic fluency.", session.TranscriptCount, session.Scenario.Name)
	}

	return domain.PracticeReport{
		SessionID:      session.ID,
		ScenarioName:   session.Scenario.Name,
		Summary:        summary,
		CEFRGuess:      cefr,
		OverallScore:   overall,
		Highlights:     buildHighlights(session),
		Corrections:    dedupeCorrections(session.HiddenFindings),
		PracticePlan:   buildPracticePlan(session.HiddenFindings, session.Scenario.ID),
		NextSession:    nextSessionSuggestion(session.Scenario.ID, cefr),
		GeneratedAt:    time.Now().UTC(),
		TotalTurns:     session.TranscriptCount,
		AverageLatency: averageLatency(session.Signals),
	}, nil
}

func fallbackTranscript(scenarioID string) string {
	switch scenarioID {
	case "interview":
		return "I worked on a project where I improved the user experience and learned to communicate better with my team."
	case "restaurant":
		return "I would like a grilled chicken salad and could you tell me if it has nuts?"
	case "meeting":
		return "This week we finished the first version and I need feedback from the design team."
	case "travel":
		return "I am looking for the train station and I need to buy a ticket."
	default:
		return "I had a good day and I would like to practice speaking more naturally."
	}
}

func nextScenarioReply(scenarioID string, userText string, turn int) string {
	shortFollowUp := "Could you add one specific detail so I can understand your point better?"
	if countWords(userText) < 6 {
		return shortFollowUp
	}

	switch scenarioID {
	case "interview":
		replies := []string{
			"That sounds useful. What was your exact role, and how did you measure success?",
			"Good. Tell me about one challenge you faced and how you handled it.",
			"If you joined our team, how would that experience help you in the first month?",
		}
		return replies[turn%len(replies)]
	case "restaurant":
		replies := []string{
			"Of course. The salad has almonds, but we can remove them. Would you like any side dish?",
			"Sure. Would you prefer still water, sparkling water, or tea with your meal?",
			"Thank you. Would you like to pay together or split the bill?",
		}
		return replies[turn%len(replies)]
	case "meeting":
		replies := []string{
			"Thanks for the update. What decision do you need from the group today?",
			"That is clear. What risk should we watch before the next milestone?",
			"Let's make this concrete. Who owns the next action item, and when is it due?",
		}
		return replies[turn%len(replies)]
	case "travel":
		replies := []string{
			"The station is about ten minutes away. Do you prefer walking directions or a taxi?",
			"You can buy a ticket at the machine. What destination should I help you choose?",
			"If your train is delayed, would you like help finding another route?",
		}
		return replies[turn%len(replies)]
	default:
		replies := []string{
			"That is interesting. What made that part of your day stand out?",
			"I see. How did you feel about it at the time?",
			"Nice. What are you looking forward to this week?",
		}
		return replies[turn%len(replies)]
	}
}

func findPronunciationIssues(text string, turn int) []domain.Correction {
	lower := strings.ToLower(text)
	issues := []domain.Correction{}
	if strings.Contains(lower, "th") {
		issues = append(issues, domain.Correction{
			Type:        "pronunciation",
			Original:    "Words with th",
			Suggestion:  "Place your tongue lightly between your teeth for /th/ sounds.",
			Explanation: "The mock evaluator detected a likely /th/ practice point.",
			Severity:    "low",
			TurnIndex:   turn,
		})
	}
	if strings.Contains(lower, "comfortable") || strings.Contains(lower, "vegetable") {
		issues = append(issues, domain.Correction{
			Type:        "pronunciation",
			Original:    "Multi-syllable word stress",
			Suggestion:  "Slow down and stress the first syllable clearly.",
			Explanation: "Long words often need deliberate stress control in spoken English.",
			Severity:    "medium",
			TurnIndex:   turn,
		})
	}
	return issues
}

func findGrammarIssues(text string, turn int) []domain.Correction {
	lower := strings.ToLower(text)
	issues := []domain.Correction{}
	patterns := []struct {
		bad         string
		good        string
		explanation string
	}{
		{"i has", "I have", "Use have with the subject I."},
		{"i am work", "I am working", "Use the -ing form after am for an ongoing action."},
		{"work in team", "work in a team", "Add an article before a singular countable noun."},
		{"can you helps", "can you help", "Use the base verb after can."},
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern.bad) {
			issues = append(issues, domain.Correction{
				Type:        "grammar",
				Original:    pattern.bad,
				Suggestion:  pattern.good,
				Explanation: pattern.explanation,
				Severity:    "medium",
				TurnIndex:   turn,
			})
		}
	}
	return issues
}

func findExpressionIssues(text string, turn int) []domain.Correction {
	issues := []domain.Correction{}
	words := countWords(text)
	if words > 0 && words < 7 {
		issues = append(issues, domain.Correction{
			Type:        "expression",
			Original:    text,
			Suggestion:  "Add one concrete detail, such as a result, reason, or example.",
			Explanation: "Short answers are understandable, but fuller answers make real conversations smoother.",
			Severity:    "low",
			TurnIndex:   turn,
		})
	}
	if strings.Contains(strings.ToLower(text), "very very") {
		issues = append(issues, domain.Correction{
			Type:        "expression",
			Original:    "very very",
			Suggestion:  "extremely / highly / especially",
			Explanation: "A precise intensifier sounds more natural than repeating very.",
			Severity:    "low",
			TurnIndex:   turn,
		})
	}
	return issues
}

func averageScores(signals []domain.TurnSignal) domain.ScoreCard {
	if len(signals) == 0 {
		return domain.ScoreCard{Pronunciation: 80, Fluency: 80, Grammar: 80, Vocabulary: 80, Interaction: 80}
	}
	var total domain.ScoreCard
	for _, signal := range signals {
		total.Pronunciation += signal.Score.Pronunciation
		total.Fluency += signal.Score.Fluency
		total.Grammar += signal.Score.Grammar
		total.Vocabulary += signal.Score.Vocabulary
		total.Interaction += signal.Score.Interaction
	}
	n := len(signals)
	return domain.ScoreCard{
		Pronunciation: total.Pronunciation / n,
		Fluency:       total.Fluency / n,
		Grammar:       total.Grammar / n,
		Vocabulary:    total.Vocabulary / n,
		Interaction:   total.Interaction / n,
	}
}

func averageLatency(signals []domain.TurnSignal) int64 {
	if len(signals) == 0 {
		return 0
	}
	var total int64
	for _, signal := range signals {
		total += signal.LatencyMs
	}
	return total / int64(len(signals))
}

func estimateCEFR(score domain.ScoreCard) string {
	avg := (score.Pronunciation + score.Fluency + score.Grammar + score.Vocabulary + score.Interaction) / 5
	switch {
	case avg >= 90:
		return "B2+"
	case avg >= 82:
		return "B1"
	case avg >= 72:
		return "A2+"
	default:
		return "A2"
	}
}

func buildHighlights(session domain.Session) []string {
	highlights := []string{"You stayed engaged with the scenario and responded to the assistant's follow-up questions."}
	if len(session.Signals) > 0 {
		last := session.Signals[len(session.Signals)-1]
		if last.Score.Interaction >= 82 {
			highlights = append(highlights, "Your interaction score shows that your answers gave the conversation enough context to continue naturally.")
		}
		if last.Score.Pronunciation >= 82 {
			highlights = append(highlights, "Your pronunciation was clear enough for the listener to follow the main idea.")
		}
	}
	return highlights
}

func buildPracticePlan(findings []domain.Correction, scenarioID string) []string {
	plan := []string{
		"Repeat your best answer once, then add one number, result, or reason.",
		"Shadow the assistant's last reply for 30 seconds to copy rhythm and linking.",
	}
	hasPronunciation := false
	hasGrammar := false
	for _, finding := range findings {
		if finding.Type == "pronunciation" {
			hasPronunciation = true
		}
		if finding.Type == "grammar" {
			hasGrammar = true
		}
	}
	if hasPronunciation {
		plan = append(plan, "Practice the collected pronunciation points slowly, then use them in a full sentence.")
	}
	if hasGrammar {
		plan = append(plan, "Rewrite each corrected sentence and speak it aloud three times.")
	}
	if scenarioID == "interview" {
		plan = append(plan, "Prepare one STAR answer with situation, task, action, and result.")
	}
	return plan
}

func nextSessionSuggestion(scenarioID string, cefr string) string {
	if scenarioID == "interview" {
		return "Try a deeper interview round focused on conflict, ownership, and measurable impact."
	}
	return "Repeat the same scenario once more, aiming for longer answers and fewer pauses."
}

func dedupeCorrections(corrections []domain.Correction) []domain.Correction {
	seen := map[string]bool{}
	result := []domain.Correction{}
	for _, correction := range corrections {
		key := correction.Type + "|" + strings.ToLower(correction.Original) + "|" + correction.Suggestion
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, correction)
	}
	return result
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func uniqueWordCount(text string) int {
	words := strings.Fields(strings.ToLower(text))
	seen := map[string]bool{}
	for _, word := range words {
		seen[strings.Trim(word, ".,!?;:\"'()")] = true
	}
	return len(seen)
}

func clamp(value int, minValue int, maxValue int) int {
	return int(math.Max(float64(minValue), math.Min(float64(maxValue), float64(value))))
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
