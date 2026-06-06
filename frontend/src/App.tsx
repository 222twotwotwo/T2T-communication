import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import {
  Activity,
  BarChart3,
  BookOpenCheck,
  CheckCircle2,
  Clock3,
  KeyRound,
  Loader2,
  Mic,
  Play,
  Save,
  Send,
  Square,
  Trash2,
  Volume2,
  X,
} from 'lucide-react';
import { createSession, finishSession, getProviderStatus, getScenarios, sendTurn } from './api';
import type { ClientKeySettings, Message, PracticeReport, ProviderStatus, Scenario, ScoreCard, SessionSnapshot, TurnSignal } from './types';

const levels = ['A2', 'B1', 'B2'];
const keySettingsStorageKey = 't2t-client-key-settings-v1';
const defaultKeySettings: ClientKeySettings = {
  llmProvider: 'openai',
  openaiKey: '',
  anthropicKey: '',
  dashScopeKey: '',
};

type SpeechRecognitionAlternativeLike = {
  transcript: string;
};

type SpeechRecognitionResultLike = {
  isFinal: boolean;
  length: number;
  [index: number]: SpeechRecognitionAlternativeLike | undefined;
};

type SpeechRecognitionEventLike = {
  resultIndex: number;
  results: ArrayLike<SpeechRecognitionResultLike>;
};

type SpeechRecognitionLike = {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  onresult: ((event: SpeechRecognitionEventLike) => void) | null;
  onerror: (() => void) | null;
  start: () => void;
  stop: () => void;
};

type SpeechRecognitionConstructor = new () => SpeechRecognitionLike;

export default function App() {
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [providerStatus, setProviderStatus] = useState<ProviderStatus | null>(null);
  const [selectedScenario, setSelectedScenario] = useState('interview');
  const [level, setLevel] = useState('B1');
  const [session, setSession] = useState<SessionSnapshot | null>(null);
  const [report, setReport] = useState<PracticeReport | null>(null);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState(false);
  const [recording, setRecording] = useState(false);
  const [error, setError] = useState('');
  const [keySettings, setKeySettings] = useState<ClientKeySettings>(() => loadKeySettings());
  const [keyDraft, setKeyDraft] = useState<ClientKeySettings>(() => loadKeySettings());
  const [keyPanelOpen, setKeyPanelOpen] = useState(false);
  const [liveTranscript, setLiveTranscript] = useState('');
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const recognitionRef = useRef<SpeechRecognitionLike | null>(null);
  const speechTranscriptRef = useRef('');
  const liveTranscriptRef = useRef('');

  useEffect(() => {
    void Promise.all([getScenarios(), getProviderStatus()])
      .then(([scenarioList, status]) => {
        setScenarios(scenarioList);
        setProviderStatus(status);
        if (scenarioList[0]) {
          setSelectedScenario(scenarioList[0].id);
        }
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  const activeScenario = useMemo(
    () => scenarios.find((scenario) => scenario.id === selectedScenario) ?? scenarios[0],
    [scenarios, selectedScenario],
  );

  const latestSignal = session?.signals.at(-1);
  const messages = session?.messages ?? [];
  const hasRuntimeKeys = Boolean(keySettings.openaiKey || keySettings.anthropicKey || keySettings.dashScopeKey);

  async function startSession() {
    setBusy(true);
    setError('');
    setReport(null);
    try {
      const created = await createSession(selectedScenario, level, keySettings);
      setSession(created);
      speak(created.messages.at(-1));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function submitTurn(event?: FormEvent) {
    event?.preventDefault();
    if (!session || !draft.trim()) {
      return;
    }
    const text = draft.trim();
    setDraft('');
    setBusy(true);
    setError('');
    try {
      const turn = await sendTurn(session.id, { text }, keySettings);
      setSession(turn.session);
      speak(turn.assistantMessage);
    } catch (err) {
      setDraft(text);
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function startRecording() {
    if (!session || recording) {
      return;
    }
    setError('');
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream);
      const recognition = createSpeechRecognition();
      chunksRef.current = [];
      speechTranscriptRef.current = '';
      liveTranscriptRef.current = '';
      setLiveTranscript('');
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        window.setTimeout(() => {
          void submitAudio();
        }, recognition ? 250 : 0);
      };
      if (recognition) {
        recognition.onresult = (event) => {
          let interimTranscript = '';
          const finalPieces: string[] = [];
          for (let index = event.resultIndex; index < event.results.length; index += 1) {
            const result = event.results[index];
            const transcript = result[0]?.transcript.trim();
            if (!transcript) {
              continue;
            }
            if (result.isFinal) {
              finalPieces.push(transcript);
            } else {
              interimTranscript = `${interimTranscript} ${transcript}`.trim();
            }
          }
          if (finalPieces.length > 0) {
            speechTranscriptRef.current = `${speechTranscriptRef.current} ${finalPieces.join(' ')}`.trim();
          }
          const preview = [speechTranscriptRef.current, interimTranscript].filter(Boolean).join(' ');
          liveTranscriptRef.current = preview;
          setLiveTranscript(preview);
        };
        recognition.onerror = () => {
          recognitionRef.current = null;
        };
        recognitionRef.current = recognition;
      }
      recorder.start();
      startSpeechRecognition(recognition);
      setRecording(true);
    } catch (err) {
      setError(`Microphone unavailable: ${(err as Error).message}`);
    }
  }

  function stopRecording() {
    const recorder = recorderRef.current;
    if (!recorder || !recording) {
      return;
    }
    stopSpeechRecognition();
    if (recorder.state !== 'inactive') {
      recorder.stop();
    }
    setRecording(false);
  }

  function stopSpeechRecognition() {
    if (!recognitionRef.current) {
      return;
    }
    const recognition = recognitionRef.current;
    recognitionRef.current = null;
    try {
      recognition.stop();
    } catch {
      // The recorder should still submit the captured audio or available transcript.
    }
  }

  async function submitAudio() {
    const transcript = (speechTranscriptRef.current || liveTranscriptRef.current).trim();
    const hasAudio = chunksRef.current.length > 0;
    if (!session || (!hasAudio && !transcript)) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      const payload: { text?: string; audioBase64?: string; mimeType?: string } = {};
      if (transcript) {
        payload.text = transcript;
      }
      if (hasAudio) {
        const blob = new Blob(chunksRef.current, { type: chunksRef.current[0]?.type || 'audio/webm' });
        payload.audioBase64 = await blobToBase64(blob);
        payload.mimeType = blob.type;
      }
      const turn = await sendTurn(session.id, payload, keySettings);
      setSession(turn.session);
      speak(turn.assistantMessage);
    } catch (err) {
      if (transcript) {
        setDraft(transcript);
      }
      setError((err as Error).message);
    } finally {
      chunksRef.current = [];
      speechTranscriptRef.current = '';
      liveTranscriptRef.current = '';
      setLiveTranscript('');
      setBusy(false);
    }
  }

  async function endSession() {
    if (!session) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      const nextReport = await finishSession(session.id, keySettings);
      setReport(nextReport);
      setSession((current) => (current ? { ...current, endedAt: nextReport.generatedAt } : current));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function openKeyPanel() {
    setKeyDraft(keySettings);
    setKeyPanelOpen(true);
  }

  function saveKeySettings() {
    const next = sanitizeKeySettings(keyDraft);
    setKeySettings(next);
    saveKeySettingsToStorage(next);
    setKeyPanelOpen(false);
  }

  function clearKeySettings() {
    setKeyDraft(defaultKeySettings);
    setKeySettings(defaultKeySettings);
    saveKeySettingsToStorage(defaultKeySettings);
  }

  return (
    <main className="app-shell">
      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">T2T Speaking Coach</p>
            <h1>Scenario speaking studio</h1>
          </div>
          <div className="topbar-actions">
            <div className="provider-chip">
              <Activity size={16} />
              <span>{providerStatus?.mode ?? 'commercial'}</span>
            </div>
            <button className={`key-button ${hasRuntimeKeys ? 'active' : ''}`} type="button" onClick={openKeyPanel} title="API keys">
              <KeyRound size={18} />
            </button>
          </div>
        </header>

        {keyPanelOpen && (
          <section className="key-panel" aria-label="api key settings">
            <div className="key-panel-heading">
              <div>
                <p className="eyebrow">Runtime keys</p>
                <h2>API keys</h2>
              </div>
              <button className="icon-button static" type="button" onClick={() => setKeyPanelOpen(false)} title="Close">
                <X size={16} />
              </button>
            </div>
            <div className="key-grid">
              <div className="field-group">
                <label htmlFor="llmProvider">LLM provider</label>
                <select
                  id="llmProvider"
                  value={keyDraft.llmProvider}
                  onChange={(event) => setKeyDraft((current) => ({ ...current, llmProvider: event.target.value as ClientKeySettings['llmProvider'] }))}
                >
                  <option value="openai">OpenAI compatible</option>
                  <option value="anthropic">Anthropic</option>
                </select>
              </div>
              <SecretField id="openaiKey" label="OpenAI key" value={keyDraft.openaiKey} onChange={(value) => setKeyDraft((current) => ({ ...current, openaiKey: value }))} />
              <SecretField id="anthropicKey" label="Anthropic key" value={keyDraft.anthropicKey} onChange={(value) => setKeyDraft((current) => ({ ...current, anthropicKey: value }))} />
              <SecretField id="dashScopeKey" label="DashScope RAG key" value={keyDraft.dashScopeKey} onChange={(value) => setKeyDraft((current) => ({ ...current, dashScopeKey: value }))} />
            </div>
            <div className="key-panel-footer">
              <span>Stored in this browser.</span>
              <div>
                <button className="ghost-action" type="button" onClick={clearKeySettings}>
                  <Trash2 size={18} />
                  <span>Clear</span>
                </button>
                <button className="primary-action" type="button" onClick={saveKeySettings}>
                  <Save size={18} />
                  <span>Save</span>
                </button>
              </div>
            </div>
          </section>
        )}

        <section className="control-panel" aria-label="session controls">
          <div className="field-group">
            <label htmlFor="scenario">Scenario</label>
            <select id="scenario" value={selectedScenario} onChange={(event) => setSelectedScenario(event.target.value)} disabled={Boolean(session && !session.endedAt)}>
              {scenarios.map((scenario) => (
                <option key={scenario.id} value={scenario.id}>
                  {scenario.name}
                </option>
              ))}
            </select>
          </div>

          <div className="segmented" aria-label="level">
            {levels.map((item) => (
              <button
                key={item}
                type="button"
                className={item === level ? 'active' : ''}
                onClick={() => setLevel(item)}
                disabled={Boolean(session && !session.endedAt)}
              >
                {item}
              </button>
            ))}
          </div>

          <button className="primary-action" type="button" onClick={startSession} disabled={busy}>
            {busy && !session ? <Loader2 className="spin" size={18} /> : <Play size={18} />}
            <span>{session && !session.endedAt ? 'Restart' : 'Start'}</span>
          </button>

          <button className="ghost-action" type="button" onClick={endSession} disabled={!session || Boolean(session.endedAt) || busy}>
            <BookOpenCheck size={18} />
            <span>Finish</span>
          </button>
        </section>

        {activeScenario && (
          <section className="scenario-strip">
            <div>
              <span>{activeScenario.userRole}</span>
              <strong>{activeScenario.name}</strong>
              <span>{activeScenario.assistantRole}</span>
            </div>
            <div className="tag-row">
              {activeScenario.tags.map((tag) => (
                <span key={tag}>{tag}</span>
              ))}
            </div>
          </section>
        )}

        <section className="practice-grid">
          <div className="conversation-panel">
            <div className="message-list" aria-live="polite">
              {messages.map((message) => (
                <article key={message.id} className={`message ${message.role}`}>
                  <span>{message.role === 'assistant' ? 'Coach' : 'You'}</span>
                  <p>{message.text}</p>
                  {message.role === 'assistant' && (
                    <button className="icon-button" type="button" onClick={() => speak(message)} title="Play voice">
                      <Volume2 size={16} />
                    </button>
                  )}
                </article>
              ))}
              {!session && <div className="empty-state">Choose a scene and start speaking.</div>}
            </div>

            {(recording || liveTranscript) && <div className="transcript-preview" aria-live="polite">{liveTranscript || 'Listening...'}</div>}

            <form className="composer" onSubmit={submitTurn}>
              <button className={`record-button ${recording ? 'recording' : ''}`} type="button" onClick={recording ? stopRecording : startRecording} disabled={!session || busy}>
                {recording ? <Square size={20} /> : <Mic size={20} />}
              </button>
              <input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="Speak or type your answer..." disabled={!session || busy || Boolean(session?.endedAt)} />
              <button className="send-button" type="submit" disabled={!session || !draft.trim() || busy || Boolean(session?.endedAt)}>
                {busy ? <Loader2 className="spin" size={18} /> : <Send size={18} />}
              </button>
            </form>
          </div>

          <aside className="insight-panel">
            <ScoreBlock title="Pronunciation" value={latestSignal?.score.pronunciation ?? 0} />
            <ScoreBlock title="Fluency" value={latestSignal?.score.fluency ?? 0} />
            <ScoreBlock title="Grammar" value={latestSignal?.score.grammar ?? 0} />
            <ScoreBlock title="Interaction" value={latestSignal?.score.interaction ?? 0} />
            <div className="metric-row">
              <Clock3 size={18} />
              <span>{latestSignal ? `${latestSignal.latencyMs} ms` : '-- ms'}</span>
            </div>
            <div className="metric-row">
              <BarChart3 size={18} />
              <span>{session?.collectedCount ?? 0} collected</span>
            </div>
          </aside>
        </section>

        {error && <div className="error-bar">{error}</div>}
      </section>

      <ReportPanel report={report} />
    </main>
  );
}

function createSpeechRecognition(): SpeechRecognitionLike | null {
  const speechWindow = window as Window & {
    SpeechRecognition?: SpeechRecognitionConstructor;
    webkitSpeechRecognition?: SpeechRecognitionConstructor;
  };
  const Recognition = speechWindow.SpeechRecognition ?? speechWindow.webkitSpeechRecognition;
  if (!Recognition) {
    return null;
  }
  const recognition = new Recognition();
  recognition.continuous = true;
  recognition.interimResults = true;
  recognition.lang = 'en-US';
  return recognition;
}

function startSpeechRecognition(recognition: SpeechRecognitionLike | null) {
  if (!recognition) {
    return;
  }
  try {
    recognition.start();
  } catch {
    // Some browsers reject duplicate or unavailable recognition starts; audio upload still works.
  }
}

function SecretField({ id, label, value, onChange }: { id: string; label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="field-group">
      <label htmlFor={id}>{label}</label>
      <input id={id} type="password" value={value} onChange={(event) => onChange(event.target.value)} autoComplete="off" spellCheck={false} />
    </div>
  );
}

function ScoreBlock({ title, value }: { title: string; value: number }) {
  return (
    <div className="score-block">
      <div className="score-meta">
        <span>{title}</span>
        <strong>{value || '--'}</strong>
      </div>
      <div className="meter" aria-hidden="true">
        <span style={{ width: `${value || 0}%` }} />
      </div>
    </div>
  );
}

function ReportPanel({ report }: { report: PracticeReport | null }) {
  const score = report?.overallScore;
  return (
    <aside className="report-panel">
      <div className="report-heading">
        <BookOpenCheck size={22} />
        <div>
          <p className="eyebrow">After-class report</p>
          <h2>{report ? report.scenarioName : 'Waiting for session'}</h2>
        </div>
      </div>

      {report ? (
        <>
          <p className="summary">{report.summary}</p>
          <div className="cefr-card">
            <span>CEFR</span>
            <strong>{report.cefrGuess}</strong>
          </div>
          {score && <ScoreTable score={score} />}
          <section className="report-section">
            <h3>Corrections</h3>
            {report.corrections.length === 0 ? (
              <p className="muted">No major issues collected.</p>
            ) : (
              report.corrections.map((correction, index) => (
                <article className="correction-item" key={`${correction.type}-${index}`}>
                  <div>
                    <span>{correction.type}</span>
                    <strong>{correction.suggestion}</strong>
                  </div>
                  <p>{correction.explanation}</p>
                </article>
              ))
            )}
          </section>
          <section className="report-section">
            <h3>Practice plan</h3>
            {report.practicePlan.map((item) => (
              <div className="plan-item" key={item}>
                <CheckCircle2 size={16} />
                <span>{item}</span>
              </div>
            ))}
          </section>
        </>
      ) : (
        <div className="report-placeholder">
          <div className="waveform" aria-hidden="true">
            {Array.from({ length: 20 }).map((_, index) => (
              <span key={index} style={{ animationDelay: `${index * 80}ms` }} />
            ))}
          </div>
        </div>
      )}
    </aside>
  );
}

function ScoreTable({ score }: { score: ScoreCard }) {
  return (
    <div className="score-table">
      {Object.entries(score).map(([key, value]) => (
        <div key={key}>
          <span>{key}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </div>
  );
}

function speak(message?: Message) {
  if (!message || message.role !== 'assistant') {
    return;
  }
  if (message.audioUrl) {
    const audio = new Audio(message.audioUrl);
    void audio.play();
    return;
  }
  if ('speechSynthesis' in window) {
    window.speechSynthesis.cancel();
    const utterance = new SpeechSynthesisUtterance(message.text);
    utterance.lang = 'en-US';
    utterance.rate = 0.96;
    window.speechSynthesis.speak(utterance);
  }
}

function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => {
      const result = String(reader.result ?? '');
      resolve(result.includes(',') ? result.split(',')[1] : result);
    };
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(blob);
  });
}

function loadKeySettings(): ClientKeySettings {
  if (typeof window === 'undefined') {
    return defaultKeySettings;
  }
  try {
    const raw = window.localStorage.getItem(keySettingsStorageKey);
    if (!raw) {
      return defaultKeySettings;
    }
    return sanitizeKeySettings(JSON.parse(raw) as Partial<ClientKeySettings>);
  } catch {
    return defaultKeySettings;
  }
}

function saveKeySettingsToStorage(settings: ClientKeySettings) {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(keySettingsStorageKey, JSON.stringify(settings));
}

function sanitizeKeySettings(settings: Partial<ClientKeySettings>): ClientKeySettings {
  const llmProvider = settings.llmProvider === 'anthropic' ? 'anthropic' : 'openai';
  return {
    llmProvider,
    openaiKey: settings.openaiKey?.trim() ?? '',
    anthropicKey: settings.anthropicKey?.trim() ?? '',
    dashScopeKey: settings.dashScopeKey?.trim() ?? '',
  };
}
