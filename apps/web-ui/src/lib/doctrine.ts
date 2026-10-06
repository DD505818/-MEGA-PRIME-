/**
 * ΩMEGA PRIME Δ — operator dashboard doctrine constants.
 *
 * Binding rules encoded here:
 * - The UI observes, explains, requests, and displays ONLY. It never executes
 *   and never fabricates balances, trades, P&L, positions, risk state,
 *   service health, or market data.
 * - Unknown state renders as UNKNOWN / BLOCKED / Unavailable / Not Implemented.
 *
 * Program banner wording and the live-lock sentence below are exact and
 * required; do not paraphrase them.
 */

/** Program evidence state. */
export const PROGRAM_STATE = 'EDGE NOT PROVEN';

/** Live deployment state. */
export const LIVE_STATE = 'LIVE LOCKED';

/**
 * Exact required wording that must accompany any LIVE LOCKED display:
 * activation requires independent certification and backend governance
 * outside this interface.
 */
export const LIVE_LOCKED_WORDING =
  'Requires independent certification and backend governance outside this interface.';

/** Combined program banner text. */
export const PROGRAM_BANNER = `${PROGRAM_STATE} · ${LIVE_STATE}`;

/**
 * Authority chain in exact order:
 * DATA → MODELS → AGENTS → MIDAS → AEGIS → VULTURE → TRUTHCORE → VALIDATION.
 */
export const AUTHORITY_CHAIN = [
  'DATA',
  'MODELS',
  'AGENTS',
  'MIDAS',
  'AEGIS',
  'VULTURE',
  'TRUTHCORE',
  'VALIDATION'
] as const;

export interface AegisGate {
  /** Gate number, 1–14. */
  n: number;
  /** Short display name. */
  name: string;
  /** Exact fail-reason codes from apps/risk-service/risk_engine.go (14 gates, never 22). */
  reason: string;
  /** One-line purpose, shown on the status home. */
  purpose: string;
}

/**
 * The 14 real AEGIS gates. Reason codes are copied verbatim from
 * apps/risk-service/risk_engine.go; gates with multiple codes list all of
 * them, and gates with dynamic suffixes (book age in ms, percentages, bps)
 * note the suffix placeholder.
 */
export const AEGIS_GATES: AegisGate[] = [
  { n: 1, name: 'Kill Switch', reason: 'GATE1_KILL_SWITCH_ACTIVE', purpose: 'Operator- or risk-engaged halt; no fills while active.' },
  { n: 2, name: 'Circuit Breaker', reason: 'GATE2_CIRCUIT_BREAKER_ACTIVE', purpose: 'Automatic halt on anomalous conditions.' },
  {
    n: 3,
    name: 'Paper/Live Mode Mismatch',
    reason: 'GATE3_NOT_IN_PAPER_MODE / GATE3_SIGNAL_MODE_MISMATCH',
    purpose: 'Rejects any signal whose mode does not match the paper-mode lock.'
  },
  { n: 4, name: 'Broker Health', reason: 'GATE4_BROKER_DOWN', purpose: 'Rejects when the broker/exchange path is down.' },
  {
    n: 5,
    name: 'Stale Market Data',
    reason: 'GATE5_NO_BOOK_TS / GATE5_BAD_BOOK_TS / GATE5_ZERO_BOOK_TS / GATE5_FUTURE_BOOK_TS_<ms> / GATE5_STALE_BOOK_<ms>',
    purpose: 'Fail-closed on missing, zero, future, or stale order-book data.'
  },
  { n: 6, name: 'Daily Loss Cap', reason: 'GATE6_DAILY_LOSS_EXCEEDED', purpose: 'Blocks new risk once the daily loss cap is hit.' },
  { n: 7, name: 'Max Drawdown', reason: 'GATE7_MAX_DRAWDOWN_EXCEEDED', purpose: 'Blocks new risk beyond the max drawdown threshold.' },
  { n: 8, name: 'Max Open Positions', reason: 'GATE8_MAX_POSITIONS_REACHED', purpose: 'Caps the number of concurrent open positions.' },
  { n: 9, name: 'Asset Exposure Cap', reason: 'GATE9_ASSET_EXPOSURE_<pct>', purpose: 'Caps exposure to a single asset relative to equity.' },
  { n: 10, name: 'Correlation Guard', reason: 'GATE10_HIGH_CORRELATION_<corr>', purpose: 'Blocks new positions highly correlated with the book.' },
  {
    n: 11,
    name: 'Duplicate Signal ID',
    reason: 'GATE11_DUPLICATE_SIGNAL_ID / GATE11_DEDUP_STORE_UNAVAILABLE',
    purpose: 'At-most-once signal dedup; refuses when the dedup store is unavailable.'
  },
  { n: 12, name: 'Spread Guard', reason: 'GATE12_SPREAD_TOO_WIDE_<bps>', purpose: 'Blocks execution when the spread exceeds the guard.' },
  { n: 13, name: 'Minimum Confidence', reason: 'GATE13_LOW_CONFIDENCE_<conf>', purpose: 'Blocks signals below the confidence floor.' },
  {
    n: 14,
    name: 'Risk Per Trade',
    reason: 'GATE14_INVALID_PRICE / GATE14_QTY_REDUCED_TO_ZERO',
    purpose: 'Rejects invalid prices and quantities reduced to zero.'
  }
];

/** Validation-lab Monte Carlo v2 engine status: merged implementation is not yet on main. */
export const MC_V2_STATUS = 'merged (PR #75) — protocol v2 is the lab';
