INSERT INTO account_state (id,equity,daily_loss,peak_equity,drawdown)
VALUES (1,100000,0,100000,0)
ON CONFLICT (id) DO NOTHING;
