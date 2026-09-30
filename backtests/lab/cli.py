"""Compatibility shim. Canonical implementation: `validation_lab.cli`.

Keeps `python -m backtests.lab.cli ...` working; prefer the canonical
`python -m validation_lab.cli ...` (or the `omega-validation-lab` script).
"""

from validation_lab.cli import (
    build_parser,
    cmd_evaluate,
    cmd_folds,
    cmd_prepare,
    cmd_verify,
    main,
)

__all__ = [
    "build_parser",
    "cmd_evaluate",
    "cmd_folds",
    "cmd_prepare",
    "cmd_verify",
    "main",
]

if __name__ == "__main__":
    main()
