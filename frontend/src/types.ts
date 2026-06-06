export type Scenario = {
  id: string;
  name: string;
  description: string;
  userRole: string;
  assistantRole: string;
  openingPrompt: string;
  tags: string[];
};

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  audioUrl?: string;
  createdAt: string;
};

export type ScoreCard = {
  pronunciation: number;
  fluency: number;
  grammar: number;
  vocabulary: number;
  interaction: number;
};

export type TurnSignal = {
  turnIndex: number;
  score: ScoreCard;
  wordsPerMinute: number;
  latencyMs: number;
  collectedIssues: number;
};

export type SessionSnapshot = {
  id: string;
  scenario: Scenario;
  level: string;
  voice: string;
  messages: Message[];
  signals: TurnSignal[];
  startedAt: string;
  endedAt?: string;
  collectedCount: number;
};

export type TurnResponse = {
  session: SessionSnapshot;
  userMessage: Message;
  assistantMessage: Message;
  signal: TurnSignal;
  transcriptSource: string;
};

export type Correction = {
  type: 'pronunciation' | 'grammar' | 'expression';
  original: string;
  suggestion: string;
  explanation: string;
  severity: 'low' | 'medium' | 'high';
  turnIndex: number;
};

export type PracticeReport = {
  sessionId: string;
  scenarioName: string;
  summary: string;
  cefrGuess: string;
  overallScore: ScoreCard;
  highlights: string[];
  corrections: Correction[];
  practicePlan: string[];
  nextSession: string;
  generatedAt: string;
  totalTurns: number;
  averageLatency: number;
};

export type ProviderStatus = {
  mode: string;
  providers: Record<string, string>;
  commercialReady: boolean;
  warnings: string[];
};

export type ClientKeySettings = {
  llmProvider: 'mock' | 'openai' | 'anthropic';
  openaiKey: string;
  anthropicKey: string;
  dashScopeKey: string;
};
