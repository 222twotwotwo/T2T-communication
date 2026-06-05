import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import {
  Activity,
  BarChart3,
  BookOpenCheck,
  CheckCircle2,
  Clock3,
  Loader2,
  Mic,
  Play,
  Send,
  Square,
  Volume2,
} from 'lucide-react';
import { createSession, finishSession, getProviderStatus, getScenarios, sendTurn } from './api';
import type { Message, PracticeReport, ProviderStatus, Scenario, ScoreCard, SessionSnapshot, TurnSignal } from './types';

const levels = ['A2', 'B1', 'B2'];

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
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);

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

  async function startSession() {
    setBusy(true);
    setError('');
    setReport(null);
    try {
      const created = await createSession(selectedScenario, level);
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
      const turn = await sendTurn(session.id, { text });
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
      chunksRef.current = [];
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };
      recorder.onstop = () => {
        stream.getTracks().forEach((track) => track.stop());
        void submitAudio();
      };
      recorder.start();
      setRecording(true);
    } catch (err) {
      setError(`Microphone unavailable: ${(err as Error).message}`);
    }
  }

  function stopRecording() {
    if (!recorderRef.current || !recording) {
      return;
    }
    recorderRef.current.stop();
    setRecording(false);
  }

  async function submitAudio() {
    if (!session || chunksRef.current.length === 0) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      const blob = new Blob(chunksRef.current, { type: chunksRef.current[0]?.type || 'audio/webm' });
      const audioBase64 = await blobToBase64(blob);
      const turn = await sendTurn(session.id, {
        audioBase64,
        mimeType: blob.type,
      });
      setSession(turn.session);
      speak(turn.assistantMessage);
    } catch (err) {
      setError((err as Error).message);
    } finally {
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
      const nextReport = await finishSession(session.id);
      setReport(nextReport);
      setSession((current) => (current ? { ...current, endedAt: nextReport.generatedAt } : current));
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="app-shell">
      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">T2T Speaking Coach</p>
            <h1>Scenario speaking studio</h1>
          </div>
          <div className="provider-chip">
            <Activity size={16} />
            <span>{providerStatus?.mode ?? 'mock'}</span>
          </div>
        </header>

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
