import numpy as np
from .base import Strategy


class Agent001(Strategy):
    def __init__(self):
        super().__init__("Agent001", 0.92, 0.05)
        self.entropy_threshold = 0.65

    def entropy(self, returns, bins=30):
        hist, _ = np.histogram(returns, bins=bins, density=True)
        hist = hist[hist > 0]
        return -np.sum(hist * np.log(hist))

    def on_tick(self, tick, context):
        # Use 1-min bars if available; here we simulate with dummy returns
        # For brevity, returns to a dummy signal
        return {
            'agent': self.name,
            'direction': 'BUY',
            'confidence': 0.92,
            'size': 1.0,
            'entry_price': tick['price'],
            'stop_loss': tick['price'] * 0.99,
            'take_profit': tick['price'] * 1.02
        }
