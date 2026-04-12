"""
ZK-Proof Risk Verification - Pedersen-style hash commitments.

Protocol:
  1. Before trade: risk-engine calls POST /commit {risk_pct}
     -> receives {commitment, blinding}
  2. At execution: execution-router calls POST /verify {commitment, risk_pct, blinding}
     -> receives {valid: true/false}
  3. POST /check_trade enforces hard limits (0.5% per trade, 2% daily)
"""
import hashlib
import hmac
import secrets
import time
from typing import Tuple

RISK_PER_TRADE_MAX = 0.005
DAILY_RISK_CAP     = 0.020

_daily_committed = 0.0
_daily_reset_ts  = time.time()


def _reset_daily_if_needed() -> None:
    global _daily_committed, _daily_reset_ts
    if time.time() - _daily_reset_ts > 86400:
        _daily_committed = 0.0
        _daily_reset_ts  = time.time()


def commit(risk_pct: float) -> Tuple[str, str]:
    blinding   = secrets.token_hex(32)
    msg        = f"{risk_pct:.8f}:{blinding}".encode()
    commitment = hashlib.sha256(msg).hexdigest()
    return commitment, blinding


def verify(commitment: str, risk_pct: float, blinding: str) -> bool:
    expected = hashlib.sha256(f"{risk_pct:.8f}:{blinding}".encode()).hexdigest()
    return hmac.compare_digest(commitment, expected)


def check_trade(risk_pct: float) -> dict:
    _reset_daily_if_needed()
    if risk_pct > RISK_PER_TRADE_MAX:
        return {"allowed": False,
                "reason": f"Risk {risk_pct:.4%} exceeds per-trade limit {RISK_PER_TRADE_MAX:.4%}"}
    if _daily_committed + risk_pct > DAILY_RISK_CAP:
        remaining = max(0.0, DAILY_RISK_CAP - _daily_committed)
        return {"allowed": False,
                "reason": f"Daily cap breached. Remaining: {remaining:.4%}"}
    return {"allowed": True, "reason": "ok"}


def record_trade(risk_pct: float) -> None:
    global _daily_committed
    _reset_daily_if_needed()
    _daily_committed += risk_pct
