import { create } from 'zustand'

export interface Tick {
  symbol: string
  price: number
  bid: number
  ask: number
  volume: number
  change_24h: number
  ts: number
}

export interface AgentState {
  name: string
  active: boolean
  winRate: number
  sharpe: number
  trades: number
  pnl: number
}

export interface Position {
  symbol: string
  side: 'LONG' | 'SHORT'
  qty: number
  entry: number
  current: number
  pnl: number
  pnlPct: number
}

export interface AnomalyEvent {
  id: string
  type: string
  severity: 'low' | 'medium' | 'high'
  message: string
  ts: number
  acknowledged: boolean
}

interface AppState {
  // Environment
  mode: 'paper' | 'live'
  setMode: (m: 'paper' | 'live') => void

  // Market data
  ticks: Record<string, Tick>
  updateTick: (tick: Tick) => void

  // Portfolio
  equity: number
  equityCurve: { ts: number; value: number }[]
  positions: Position[]
  dailyPnl: number
  setEquity: (v: number) => void

  // Risk
  dailyLossPct: number
  drawdownPct: number
  varValue: number
  killSwitchActive: boolean
  setKillSwitch: (v: boolean) => void

  // Agents
  agents: AgentState[]
  toggleAgent: (name: string) => void

  // Anomalies
  anomalies: AnomalyEvent[]
  acknowledgeAnomaly: (id: string) => void

  // WebSocket
  wsConnected: boolean
  wsLatency: number
  setWsStatus: (connected: boolean, latency: number) => void

  // Onboarding
  onboardingComplete: boolean
  setOnboardingComplete: (v: boolean) => void
}

const AGENT_NAMES = [
  'ceo','cfo','mamba','surge','arb','opt','gold','guard','maker',
  'twin','nexus','senti','orbit','oracle','harvest','stake','mev-hunter','midas','hi-darts',
]

const initialAgents: AgentState[] = AGENT_NAMES.map((name, i) => ({
  name,
  active: i > 1,  // CEO and CFO always active; others active by default
  winRate: 52 + Math.random() * 25,
  sharpe: 1.2 + Math.random() * 2.5,
  trades: Math.floor(Math.random() * 200),
  pnl: (Math.random() - 0.3) * 5000,
}))

export const useAppStore = create<AppState>((set) => ({
  mode: 'paper',
  setMode: (m) => set({ mode: m }),

  ticks: {},
  updateTick: (tick) => set((s) => ({ ticks: { ...s.ticks, [tick.symbol]: tick } })),

  equity: 10000,
  equityCurve: Array.from({ length: 50 }, (_, i) => ({
    ts: Date.now() - (50 - i) * 3600000,
    value: 10000 * (1 + i * 0.006 + Math.random() * 0.02 - 0.01),
  })),
  positions: [
    { symbol: 'BTC/USD', side: 'LONG', qty: 0.25, entry: 62100, current: 64231, pnl: 532.75, pnlPct: 3.43 },
    { symbol: 'ETH/USD', side: 'LONG', qty: 2.5,  entry: 3050,  current: 3142,  pnl: 230.0,  pnlPct: 3.02 },
    { symbol: 'SOL/USD', side: 'SHORT', qty: 10,  entry: 180,   current: 172,   pnl: 80.0,   pnlPct: 4.44 },
  ],
  dailyPnl: 842.75,
  setEquity: (v) => set({ equity: v }),

  dailyLossPct: 0.84,
  drawdownPct: 2.1,
  varValue: 312.5,
  killSwitchActive: false,
  setKillSwitch: (v) => set({ killSwitchActive: v }),

  agents: initialAgents,
  toggleAgent: (name) => set((s) => ({
    agents: s.agents.map((a) => a.name === name ? { ...a, active: !a.active } : a),
  })),

  anomalies: [
    { id: '1', type: 'LATENCY_SPIKE', severity: 'medium', message: 'Gateway latency 142ms (threshold: 100ms)', ts: Date.now() - 120000, acknowledged: false },
    { id: '2', type: 'VOL_REGIME',    severity: 'low',    message: 'BTC volatility entering high-vol regime', ts: Date.now() - 600000,  acknowledged: false },
  ],
  acknowledgeAnomaly: (id) => set((s) => ({
    anomalies: s.anomalies.map((a) => a.id === id ? { ...a, acknowledged: true } : a),
  })),

  wsConnected: false,
  wsLatency: 0,
  setWsStatus: (connected, latency) => set({ wsConnected: connected, wsLatency: latency }),

  onboardingComplete: localStorage.getItem('op_onboarded') === 'true',
  setOnboardingComplete: (v) => {
    localStorage.setItem('op_onboarded', String(v))
    set({ onboardingComplete: v })
  },
}))
