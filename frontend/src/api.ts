import type { ClientKeySettings, PracticeReport, ProviderStatus, RAGIngestResponse, Scenario, SessionSnapshot, TurnResponse } from './types';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options?.headers ?? {}),
    },
  });

  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error ?? response.statusText);
  }

  return response.json() as Promise<T>;
}

export function getScenarios() {
  return request<Scenario[]>('/api/scenarios');
}

export function getProviderStatus() {
  return request<ProviderStatus>('/api/provider-status');
}

export function sendTurn(sessionId: string, payload: { text?: string; audioBase64?: string; mimeType?: string }, keys?: ClientKeySettings) {
  return request<TurnResponse>(`/api/sessions/${sessionId}/turn`, {
    method: 'POST',
    headers: credentialHeaders(keys),
    body: JSON.stringify(payload),
  });
}

export function finishSession(sessionId: string, keys?: ClientKeySettings) {
  return request<PracticeReport>(`/api/sessions/${sessionId}/finish`, {
    method: 'POST',
    headers: credentialHeaders(keys),
    body: JSON.stringify({}),
  });
}

export function createSession(scenarioId: string, level: string, keys?: ClientKeySettings) {
  return request<SessionSnapshot>('/api/sessions', {
    method: 'POST',
    headers: credentialHeaders(keys),
    body: JSON.stringify({ scenarioId, level }),
  });
}

export async function ingestRAGFile(category: string, file: File, keys?: ClientKeySettings) {
  const form = new FormData();
  form.append('category', category);
  form.append('file', file);

  const response = await fetch('/api/rag/ingest', {
    method: 'POST',
    headers: credentialHeaders(keys),
    body: form,
  });

  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error ?? response.statusText);
  }

  return response.json() as Promise<RAGIngestResponse>;
}

function credentialHeaders(keys?: ClientKeySettings): Record<string, string> {
  const headers: Record<string, string> = {};
  if (!keys) {
    return headers;
  }
  if (keys.llmProvider) {
    headers['X-T2T-LLM-Provider'] = keys.llmProvider;
  }
  if (keys.openaiKey.trim()) {
    headers['X-T2T-OpenAI-Key'] = keys.openaiKey.trim();
  }
  if (keys.anthropicKey.trim()) {
    headers['X-T2T-Anthropic-Key'] = keys.anthropicKey.trim();
  }
  if (keys.dashScopeKey.trim()) {
    headers['X-T2T-DashScope-Key'] = keys.dashScopeKey.trim();
  }
  return headers;
}
