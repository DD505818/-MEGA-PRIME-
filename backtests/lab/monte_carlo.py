"""Compatibility shim.

Canonical implementation: `validation_lab.bootstrap.empirical_block_bootstrap`
(the empirical path bootstrap lives with the other bootstraps; the
parametric Monte Carlo generators are in `validation_lab.montecarlo`).
"""

from validation_lab.bootstrap import empirical_block_bootstrap

__all__ = ["empirical_block_bootstrap"]
