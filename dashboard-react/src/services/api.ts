const BASE = (import.meta.env.VITE_GATEWAY_URL as string) || 'http://localhost:8080'

async function req<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  if (!res.ok) throw new Error(`API error ${res.status}: ${path}`)
  return res.json() as Promise<T>
}

export const api = {
  health:        ()             => req<{ status: string }>('/health'),
  routes:        ()             => req<{ services: string[]; agents: string[] }>('/routes'),
  wsStats:       ()             => req<{ connections: number; uptime_s: number }>('/ws/stats'),
  killSwitch:    ()             => req('/api/v1/kill', { method: 'POST' }),
  activateAgent: (agent: string) => req('/activate', { method: 'POST', body: JSON.stringify({ agent }) }),
  deactivateAgent: (agent: string) => req('/deactivate', { method: 'POST', body: JSON.stringify({ agent }) }),
  ceoStatus:     ()             => req('/status'),
  cfoBudget:     ()             => req('/budget'),
  zkCommit:      (risk_pct: number) =>
    req<{ commitment: string; blinding: string }>('/zk/commit', { method: 'POST', body: JSON.stringify({ risk_pct }) }),
  llmChat:       (message: string) =>
    req<{ reply: string }>('/api/v1/llm/chat', { method: 'POST', body: JSON.stringify({ message }) }),
}
