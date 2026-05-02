-- ΩMEGA PRIME Δ — PostgreSQL / TimescaleDB schema
-- Append-only design; no UPDATEs on audit tables.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ── Audit log (Truth-Core hash chain) ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL PRIMARY KEY,
    entry_id    UUID        NOT NULL UNIQUE DEFAULT uuid_generate_v4(),
    event_type  TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    prev_hash   TEXT        NOT NULL,
    hash        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_event   ON audit_log(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_log(created_at DESC);

-- ── Signals ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS signals (
    signal_id       UUID        PRIMARY KEY,
    strategy_id     TEXT        NOT NULL,
    symbol          TEXT        NOT NULL,
    side            TEXT        NOT NULL,
    quantity        NUMERIC(20,8) NOT NULL,
    limit_price     NUMERIC(20,8) NOT NULL,
    stop_price      NUMERIC(20,8),
    target_price    NUMERIC(20,8),
    confidence      NUMERIC(5,4) NOT NULL,
    mode            TEXT        NOT NULL DEFAULT 'paper',
    reason          TEXT,
    risk_approved   BOOLEAN     DEFAULT FALSE,
    reject_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SELECT create_hypertable('signals', 'created_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_signals_strategy ON signals(strategy_id);
CREATE INDEX IF NOT EXISTS idx_signals_symbol   ON signals(symbol);

-- ── Orders ───────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS orders (
    order_id        UUID        PRIMARY KEY,
    signal_id       UUID        REFERENCES signals(signal_id),
    strategy_id     TEXT        NOT NULL,
    symbol          TEXT        NOT NULL,
    side            TEXT        NOT NULL,
    quantity        NUMERIC(20,8) NOT NULL,
    limit_price     NUMERIC(20,8) NOT NULL,
    order_type      TEXT        NOT NULL,
    state           TEXT        NOT NULL,
    venue           TEXT,
    filled_qty      NUMERIC(20,8) DEFAULT 0,
    avg_fill_price  NUMERIC(20,8) DEFAULT 0,
    slippage_bps    NUMERIC(10,4) DEFAULT 0,
    meta            JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SELECT create_hypertable('orders', 'created_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_orders_signal   ON orders(signal_id);
CREATE INDEX IF NOT EXISTS idx_orders_symbol   ON orders(symbol);
CREATE INDEX IF NOT EXISTS idx_orders_state    ON orders(state);

-- ── Fills ────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS fills (
    fill_id         BIGSERIAL PRIMARY KEY,
    order_id        UUID        NOT NULL,
    signal_id       UUID,
    strategy_id     TEXT        NOT NULL,
    symbol          TEXT        NOT NULL,
    side            TEXT        NOT NULL,
    quantity        NUMERIC(20,8) NOT NULL,
    fill_price      NUMERIC(20,8) NOT NULL,
    slippage_bps    NUMERIC(10,4) DEFAULT 0,
    venue           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SELECT create_hypertable('fills', 'created_at', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_fills_order    ON fills(order_id);
CREATE INDEX IF NOT EXISTS idx_fills_strategy ON fills(strategy_id);

-- ── Portfolio snapshots ───────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    equity          NUMERIC(20,4) NOT NULL,
    peak_equity     NUMERIC(20,4) NOT NULL,
    daily_pnl       NUMERIC(20,4) NOT NULL,
    total_pnl       NUMERIC(20,4) NOT NULL,
    drawdown_pct    NUMERIC(8,6)  NOT NULL,
    open_positions  INTEGER       NOT NULL DEFAULT 0,
    snapshot_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
SELECT create_hypertable('portfolio_snapshots', 'snapshot_at', if_not_exists => TRUE);

-- ── Strategy KPIs ────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS strategy_kpis (
    id              BIGSERIAL PRIMARY KEY,
    strategy_id     TEXT        NOT NULL,
    sharpe_30d      NUMERIC(8,4),
    win_rate        NUMERIC(5,4),
    avg_win         NUMERIC(20,8),
    avg_loss        NUMERIC(20,8),
    profit_factor   NUMERIC(8,4),
    max_drawdown    NUMERIC(8,6),
    total_trades    INTEGER     DEFAULT 0,
    consecutive_losses INTEGER  DEFAULT 0,
    alpha_decayed   BOOLEAN     DEFAULT FALSE,
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_kpi_strategy ON strategy_kpis(strategy_id);

-- ── Positions (current) ──────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS positions (
    symbol          TEXT        PRIMARY KEY,
    quantity        NUMERIC(20,8) NOT NULL DEFAULT 0,
    avg_cost        NUMERIC(20,8) NOT NULL DEFAULT 0,
    market_value    NUMERIC(20,8) NOT NULL DEFAULT 0,
    unrealized_pnl  NUMERIC(20,8) NOT NULL DEFAULT 0,
    realized_pnl    NUMERIC(20,8) NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Initial seed data ─────────────────────────────────────────────────────────
INSERT INTO portfolio_snapshots (equity, peak_equity, daily_pnl, total_pnl, drawdown_pct, open_positions)
VALUES (100000, 100000, 0, 0, 0, 0)
ON CONFLICT DO NOTHING;
