"""MIDAS promotion/demotion thresholds.

Every bar is read from the environment with a documented default. The
defaults mirror the validation lab's ``LabConfig`` so the lab and the gate
agree out of the box. Changing the bar is config, not code -- but raising it
is the only safe direction; lowering it weakens the promotion gate.

Promotion bar (must be met or exceeded by the report's own config):
  MIDAS_MIN_NET_SHARPE        default 0.0
  MIDAS_MIN_OOS_SHARPE        default 0.5
  MIDAS_MIN_POSITIVE_FOLD_FRAC default 0.6
  MIDAS_MAX_PVALUE            default 0.05
  MIDAS_MIN_DSR               default 0.95
  MIDAS_SENSITIVITY_MIN_FRAC  default 0.5
  MIDAS_MAX_MC_LOSS_PROB      default 0.20

Demotion bar (PAPER outcomes; breaching either demotes a PROMOTED strategy):
  MIDAS_DEMOTE_MIN_SHARPE     default 0.0   (trailing per-trade Sharpe floor)
  MIDAS_DEMOTE_MAX_DRAWDOWN   default 0.25  (max drawdown on cumulative PnL)
  MIDAS_DEMOTE_MIN_FILLS      default 30    (fills needed before evaluation)
  MIDAS_DEMOTE_TRADES_PER_YEAR default 252.0 (annualization for per-trade Sharpe)

Paths:
  MIDAS_DB_PATH     SQLite registry file.
                    default ~/.omega-prime/midas.db
  MIDAS_DATASET_DIR frozen dataset root holding */MANIFEST.json.
                    default ~/workspace/datasets/omega-prime
"""

from __future__ import annotations

import os
from dataclasses import dataclass


def _f(name: str, default: float) -> float:
    try:
        return float(os.getenv(name, default))
    except (TypeError, ValueError):
        return default


def _i(name: str, default: int) -> int:
    try:
        return int(float(os.getenv(name, default)))
    except (TypeError, ValueError):
        return default


@dataclass(frozen=True)
class Thresholds:
    # promotion bar (mirrors validation_lab.LabConfig)
    min_net_sharpe: float = 0.0
    min_oos_sharpe: float = 0.5
    min_positive_fold_frac: float = 0.6
    max_pvalue: float = 0.05
    min_dsr: float = 0.95
    sensitivity_min_frac: float = 0.5
    max_mc_loss_prob: float = 0.20
    # demotion bar (PAPER outcomes)
    demote_min_sharpe: float = 0.0
    demote_max_drawdown: float = 0.25
    demote_min_fills: int = 30
    demote_trades_per_year: float = 252.0
    # paths
    db_path: str = ""
    dataset_dir: str = ""

    @classmethod
    def from_env(cls) -> "Thresholds":
        return cls(
            min_net_sharpe=_f("MIDAS_MIN_NET_SHARPE", 0.0),
            min_oos_sharpe=_f("MIDAS_MIN_OOS_SHARPE", 0.5),
            min_positive_fold_frac=_f("MIDAS_MIN_POSITIVE_FOLD_FRAC", 0.6),
            max_pvalue=_f("MIDAS_MAX_PVALUE", 0.05),
            min_dsr=_f("MIDAS_MIN_DSR", 0.95),
            sensitivity_min_frac=_f("MIDAS_SENSITIVITY_MIN_FRAC", 0.5),
            max_mc_loss_prob=_f("MIDAS_MAX_MC_LOSS_PROB", 0.20),
            demote_min_sharpe=_f("MIDAS_DEMOTE_MIN_SHARPE", 0.0),
            demote_max_drawdown=_f("MIDAS_DEMOTE_MAX_DRAWDOWN", 0.25),
            demote_min_fills=_i("MIDAS_DEMOTE_MIN_FILLS", 30),
            demote_trades_per_year=_f("MIDAS_DEMOTE_TRADES_PER_YEAR", 252.0),
            db_path=os.getenv(
                "MIDAS_DB_PATH",
                os.path.join(os.path.expanduser("~"), ".omega-prime", "midas.db"),
            ),
            dataset_dir=os.path.expanduser(
                os.getenv("MIDAS_DATASET_DIR", "~/workspace/datasets/omega-prime")
            ),
        )
