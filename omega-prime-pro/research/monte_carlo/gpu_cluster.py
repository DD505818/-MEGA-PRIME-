import numpy as np

def run_paths(n_paths: int = 50000, n_steps: int = 252, mu: float = 0.1, sigma: float = 0.2):
    dt = 1 / n_steps
    shocks = np.random.normal((mu - 0.5 * sigma**2) * dt, sigma * np.sqrt(dt), (n_paths, n_steps))
    return np.exp(shocks.cumsum(axis=1))
