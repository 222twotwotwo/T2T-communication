import type { PracticeReport, ProviderStatus, Scenario, SessionSnapshot, TurnResponse } from './types';

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

export function createSession(scenarioId: string, level: string) {
  return request<SessionSnapshot>('/api/sessions', {
    method: 'POST',
    body: JSON.stringify({ scenarioId, level }),
  });
}

export function sendTurn(sessionId: string, payload: { text?: string; audioBase64?: string; mimeType?: string }) {
  return request<TurnResponse>(`/api/sessions/${sessionId}/turn`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function finishSession(sessionId: string) {
  return request<PracticeReport>(`/api/sessions/${sessionId}/finish`, {
    method: 'POST',
    body: JSON.stringify({}),
  });
}
