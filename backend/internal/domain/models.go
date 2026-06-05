package domain

import "time"

type Scenario struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	UserRole      string   `json:"userRole"`
	AssistantRole string   `json:"assistantRole"`
	OpeningPrompt string   `json:"openingPrompt"`
	Tags          []string `json:"tags"`
}

type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	AudioURL  string    `json:"audioUrl,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type ScoreCard struct {
	Pronunciation int `json:"pronunciation"`
	Fluency       int `json:"fluency"`
	Grammar       int `json:"grammar"`
	Vocabulary    int `json:"vocabulary"`
	Interaction   int `json:"interaction"`
}

type TurnSignal struct {
	TurnIndex       int       `json:"turnIndex"`
	Score           ScoreCard `json:"score"`
	WordsPerMinute  int       `json:"wordsPerMinute"`
	LatencyMs       int64     `json:"latencyMs"`
	CollectedIssues int       `json:"collectedIssues"`
}

type Correction struct {
	Type        string `json:"type"`
	Original    string `json:"original"`
	Suggestion  string `json:"suggestion"`
	Explanation string `json:"explanation"`
	Severity    string `json:"severity"`
	TurnIndex   int    `json:"turnIndex"`
}

type Session struct {
	ID              string       `json:"id"`
	Scenario        Scenario     `json:"scenario"`
	Level           string       `json:"level"`
	Voice           string       `json:"voice"`
	Messages        []Message    `json:"messages"`
	Signals         []TurnSignal `json:"signals"`
	HiddenFindings  []Correction `json:"hiddenFindings,omitempty"`
	StartedAt       time.Time    `json:"startedAt"`
	EndedAt         *time.Time   `json:"endedAt,omitempty"`
	LastActivityAt  time.Time    `json:"lastActivityAt"`
	TranscriptCount int          `json:"transcriptCount"`
}

type SessionSnapshot struct {
	ID             string       `json:"id"`
	Scenario       Scenario     `json:"scenario"`
	Level          string       `json:"level"`
	Voice          string       `json:"voice"`
	Messages       []Message    `json:"messages"`
	Signals        []TurnSignal `json:"signals"`
	StartedAt      time.Time    `json:"startedAt"`
	EndedAt        *time.Time   `json:"endedAt,omitempty"`
	CollectedCount int          `json:"collectedCount"`
}

type PracticeReport struct {
	SessionID      string       `json:"sessionId"`
	ScenarioName   string       `json:"scenarioName"`
	Summary        string       `json:"summary"`
	CEFRGuess      string       `json:"cefrGuess"`
	OverallScore   ScoreCard    `json:"overallScore"`
	Highlights     []string     `json:"highlights"`
	Corrections    []Correction `json:"corrections"`
	PracticePlan   []string     `json:"practicePlan"`
	NextSession    string       `json:"nextSession"`
	GeneratedAt    time.Time    `json:"generatedAt"`
	TotalTurns     int          `json:"totalTurns"`
	AverageLatency int64        `json:"averageLatency"`
}

type ASRRequest struct {
	AudioBase64 string `json:"audioBase64"`
	MimeType    string `json:"mimeType"`
	HintText    string `json:"hintText"`
	ScenarioID  string `json:"scenarioId"`
}

type ASRResult struct {
	Transcript string  `json:"transcript"`
	Confidence float64 `json:"confidence"`
	Provider   string  `json:"provider"`
}

type PronunciationRequest struct {
	Text        string `json:"text"`
	AudioBase64 string `json:"audioBase64"`
	MimeType    string `json:"mimeType"`
	ScenarioID  string `json:"scenarioId"`
	TurnIndex   int    `json:"turnIndex"`
}

type PronunciationResult struct {
	Score    ScoreCard    `json:"score"`
	Findings []Correction `json:"findings"`
	Provider string       `json:"provider"`
}

type TTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
}

type TTSResult struct {
	AudioBase64 string `json:"audioBase64"`
	MimeType    string `json:"mimeType"`
	Provider    string `json:"provider"`
}

type ConversationContext struct {
	Session   Session   `json:"session"`
	UserText  string    `json:"userText"`
	TurnIndex int       `json:"turnIndex"`
	StartedAt time.Time `json:"startedAt"`
}

type LLMReply struct {
	Text              string       `json:"text"`
	HiddenCorrections []Correction `json:"hiddenCorrections"`
	Provider          string       `json:"provider"`
}

type ReportContext struct {
	Session Session `json:"session"`
}
