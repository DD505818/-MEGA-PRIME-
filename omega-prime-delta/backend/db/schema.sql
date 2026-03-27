CREATE TABLE IF NOT EXISTS leader_election (
  id INT PRIMARY KEY,
  leader_id TEXT NOT NULL,
  last_heartbeat TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS account_state (
  id INT PRIMARY KEY,
  equity DOUBLE PRECISION,
  daily_loss DOUBLE PRECISION,
  peak_equity DOUBLE PRECISION,
  drawdown DOUBLE PRECISION
);
