"""Durable promotion registry (SQLite).

State machine per strategy_id:
  CANDIDATE -> VALIDATING -> PROMOTED -> DEMOTED -> (fresh PASS) -> PROMOTED
  CANDIDATE -> VALIDATING -> REJECTED  (may be re-evaluated with any report)

Unknown strategy_id is treated as NOT promoted -- the gate fails closed.
"""

from __future__ import annotations

import json
import os
import sqlite3
import threading
import time

CANDIDATE = "CANDIDATE"
VALIDATING = "VALIDATING"
PROMOTED = "PROMOTED"
REJECTED = "REJECTED"
DEMOTED = "DEMOTED"

STATUSES = (CANDIDATE, VALIDATING, PROMOTED, REJECTED, DEMOTED)

_SCHEMA = """
CREATE TABLE IF NOT EXISTS strategies (
  strategy_id      TEXT PRIMARY KEY,
  status           TEXT NOT NULL,
  promoted_at      TEXT,
  verdict          TEXT,
  report_sha256    TEXT,
  data_sha256      TEXT,
  metrics_json     TEXT,
  promoted_by      TEXT,
  reject_reason    TEXT,
  demotion_history TEXT NOT NULL DEFAULT '[]',
  updated_at       TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS fills (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  strategy_id TEXT NOT NULL,
  ts          TEXT NOT NULL,
  pnl         REAL NOT NULL,
  notional    REAL
);
CREATE INDEX IF NOT EXISTS idx_fills_strategy ON fills(strategy_id, id);
"""


def _utcnow() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


class PromotionRegistry:
    """SQLite-backed strategy promotion state. Fail closed on any error."""

    def __init__(self, path: str | None = None):
        from .thresholds import Thresholds

        self.path = path or Thresholds.from_env().db_path
        parent = os.path.dirname(os.path.abspath(self.path))
        if parent:
            os.makedirs(parent, exist_ok=True)
        self._lock = threading.Lock()
        self._db = sqlite3.connect(self.path, check_same_thread=False)
        with self._lock, self._db:
            self._db.executescript(_SCHEMA)

    # ---- reads ----

    def get(self, strategy_id: str) -> dict | None:
        with self._lock:
            row = self._db.execute(
                "SELECT strategy_id, status, promoted_at, verdict, report_sha256,"
                " data_sha256, metrics_json, promoted_by, reject_reason,"
                " demotion_history, updated_at FROM strategies WHERE strategy_id = ?",
                (strategy_id,),
            ).fetchone()
        if row is None:
            return None
        rec = {
            "strategy_id": row[0],
            "status": row[1],
            "promoted_at": row[2],
            "verdict": row[3],
            "report_sha256": row[4],
            "data_sha256": row[5],
            "metrics": json.loads(row[6]) if row[6] else {},
            "promoted_by": row[7],
            "reject_reason": row[8],
            "demotion_history": json.loads(row[9]) if row[9] else [],
            "updated_at": row[10],
        }
        return rec

    def is_promoted(self, strategy_id: str) -> bool:
        """True only for PROMOTED. Unknown ids, NULLs, errors -> False."""
        try:
            rec = self.get(strategy_id)
        except Exception:
            return False
        return bool(rec) and rec["status"] == PROMOTED

    def list(self) -> list[dict]:
        with self._lock:
            ids = self._db.execute(
                "SELECT strategy_id FROM strategies ORDER BY strategy_id"
            ).fetchall()
        return [self.get(r[0]) for r in ids]

    def used_report_hashes(self, strategy_id: str) -> set[str]:
        """Report hashes consumed by prior promotions of this strategy."""
        rec = self.get(strategy_id)
        used: set[str] = set()
        if rec:
            if rec.get("report_sha256"):
                used.add(rec["report_sha256"])
            for ev in rec.get("demotion_history", []):
                h = ev.get("report_sha256")
                if h:
                    used.add(h)
        return used

    # ---- writes ----

    def _upsert(self, strategy_id: str, **fields) -> None:
        fields = dict(fields)
        fields["updated_at"] = _utcnow()
        with self._lock, self._db:
            cur = self._db.execute(
                "SELECT 1 FROM strategies WHERE strategy_id = ?", (strategy_id,)
            ).fetchone()
            if cur is None:
                cols = ["strategy_id"] + list(fields.keys())
                vals = [strategy_id] + list(fields.values())
                self._db.execute(
                    f"INSERT INTO strategies ({', '.join(cols)}) "
                    f"VALUES ({', '.join('?' * len(cols))})",
                    vals,
                )
            else:
                sets = ", ".join(f"{k} = ?" for k in fields)
                self._db.execute(
                    f"UPDATE strategies SET {sets} WHERE strategy_id = ?",
                    list(fields.values()) + [strategy_id],
                )

    def set_validating(self, strategy_id: str) -> None:
        cur = self.get(strategy_id)
        if cur is None:
            self._upsert(strategy_id, status=CANDIDATE)
        self._upsert(strategy_id, status=VALIDATING, reject_reason=None)

    def set_promoted(
        self,
        strategy_id: str,
        *,
        verdict: str,
        report_sha256: str,
        data_sha256: str,
        metrics: dict,
        promoted_by: str | None,
    ) -> None:
        self._upsert(
            strategy_id,
            status=PROMOTED,
            promoted_at=_utcnow(),
            verdict=verdict,
            report_sha256=report_sha256,
            data_sha256=data_sha256,
            metrics_json=json.dumps(metrics, sort_keys=True),
            promoted_by=promoted_by,
            reject_reason=None,
        )

    def set_rejected(self, strategy_id: str, reason: str) -> None:
        self._upsert(
            strategy_id,
            status=REJECTED,
            verdict=None,
            reject_reason=reason[:2000],
        )

    def record_reject_reason(self, strategy_id: str, reason: str) -> None:
        """Record a rejection reason without changing status.

        Used when a re-promotion attempt fails for a DEMOTED strategy: the
        PAPER failure stands, so the status must not flip to REJECTED.
        """
        self._upsert(strategy_id, reject_reason=reason[:2000])

    def set_demoted(self, strategy_id: str, reason: str, report_sha256: str | None) -> None:
        rec = self.get(strategy_id) or {}
        history = list(rec.get("demotion_history", []))
        history.append(
            {
                "ts": _utcnow(),
                "reason": reason[:2000],
                "report_sha256": report_sha256,
            }
        )
        self._upsert(
            strategy_id,
            status=DEMOTED,
            demotion_history=json.dumps(history),
            reject_reason=None,
        )

    # ---- PAPER fills ----

    def record_fill(
        self,
        strategy_id: str,
        pnl: float,
        ts: str | None = None,
        notional: float | None = None,
    ) -> None:
        with self._lock, self._db:
            self._db.execute(
                "INSERT INTO fills (strategy_id, ts, pnl, notional) VALUES (?, ?, ?, ?)",
                (strategy_id, ts or _utcnow(), float(pnl), notional),
            )

    def fills(self, strategy_id: str, limit: int | None = None) -> list[float]:
        q = "SELECT pnl FROM fills WHERE strategy_id = ? ORDER BY id"
        args: tuple = (strategy_id,)
        if limit is not None:
            q += " LIMIT ?"
            args = (strategy_id, int(limit))
        with self._lock:
            rows = self._db.execute(q, args).fetchall()
        return [r[0] for r in rows]

    def close(self) -> None:
        with self._lock:
            self._db.close()
