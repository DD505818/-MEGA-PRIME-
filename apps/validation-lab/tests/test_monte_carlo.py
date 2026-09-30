
import numpy as np

from validation_lab.bootstrap import empirical_block_bootstrap


def test_bootstrap_reproducible_with_fixed_seed():
    rng = np.random.default_rng(9)
    returns = rng.normal(0.0001, 0.01, size=120)
    a = empirical_block_bootstrap(returns, simulations=50, block_size=10, seed=11)
    b = empirical_block_bootstrap(returns, simulations=50, block_size=10, seed=11)
    assert a == b
    assert 0.0 <= a["ruin_probability"] <= 1.0
