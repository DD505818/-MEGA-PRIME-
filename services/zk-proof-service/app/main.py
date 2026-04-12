import time
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from . import zk

app = FastAPI(title="zk-proof-service", description="ZK-proof risk verification")

class CommitRequest(BaseModel):
    risk_pct: float = Field(..., gt=0, le=0.05, description="Risk as fraction, e.g. 0.005 = 0.5%")

class VerifyRequest(BaseModel):
    commitment: str
    risk_pct: float
    blinding: str

class CheckRequest(BaseModel):
    risk_pct: float
    symbol: str = "BTC/USD"

@app.get("/health")
def health():
    return {"service": "zk-proof-service", "status": "ok"}

@app.post("/commit")
def commit_risk(req: CommitRequest):
    commitment, blinding = zk.commit(req.risk_pct)
    return {"commitment": commitment, "blinding": blinding, "risk_pct": req.risk_pct}

@app.post("/verify")
def verify_risk(req: VerifyRequest):
    valid = zk.verify(req.commitment, req.risk_pct, req.blinding)
    return {"valid": valid, "risk_pct": req.risk_pct}

@app.post("/check_trade")
def check_trade(req: CheckRequest):
    result = zk.check_trade(req.risk_pct)
    if result["allowed"]:
        zk.record_trade(req.risk_pct)
    return {**result, "symbol": req.symbol, "ts": time.time()}

@app.get("/daily_usage")
def daily_usage():
    zk._reset_daily_if_needed()
    return {
        "daily_committed": zk._daily_committed,
        "daily_cap": zk.DAILY_RISK_CAP,
        "remaining": max(0.0, zk.DAILY_RISK_CAP - zk._daily_committed),
        "reset_ts": zk._daily_reset_ts,
    }
