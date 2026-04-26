export type Mode = 'backtest' | 'paper' | 'live';
export type Role = 'admin' | 'operator' | 'trader' | 'viewer';

export interface EventEnvelope<T> {
  ts: string;
  stale: boolean;
  payload: T;
}

export interface PortfolioSnapshot {
  equity: number;
  nav: number;
  dailyPnl: number;
  drawdown: number;
  exposure: number;
}
