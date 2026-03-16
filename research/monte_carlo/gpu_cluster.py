import numpy as np

def run_paths(initial_price: float, mu: float, sigma: float, steps: int = 390, n_paths: int = 50000):
    dt = 1 / 252 / steps
    shocks = np.random.normal((mu - 0.5 * sigma ** 2) * dt, sigma * np.sqrt(dt), size=(n_paths, steps))
    return initial_price * np.exp(np.cumsum(shocks, axis=1))
