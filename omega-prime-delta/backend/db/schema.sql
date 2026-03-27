CREATE TABLE IF NOT EXISTS account_state (
  id INTEGER PRIMARY KEY,
  equity DOUBLE PRECISION NOT NULL DEFAULT 0,
  daily_loss DOUBLE PRECISION NOT NULL DEFAULT 0,
  peak_equity DOUBLE PRECISION NOT NULL DEFAULT 0,
  drawdown DOUBLE PRECISION NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS leader_election (
  id INTEGER PRIMARY KEY,
  leader_id TEXT NOT NULL,
  last_heartbeat TIMESTAMPTZ NOT NULL
);

INSERT INTO account_state (id, equity, daily_loss, peak_equity, drawdown)
VALUES (1, 10000, 0, 10000, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO leader_election (id, leader_id, last_heartbeat)
VALUES (1, 'bootstrap', NOW())
ON CONFLICT (id) DO NOTHING;
