from .base import Strategy


class Agent002(Strategy):
    def __init__(self):
        super().__init__("Agent002", 0.80, 0.05)

    def on_tick(self, tick, context):
        return {
            'agent': self.name,
            'direction': 'BUY',
            'confidence': self.win_rate,
            'size': 1.0,
            'entry_price': tick['price'],
            'stop_loss': tick['price'] * 0.99,
            'take_profit': tick['price'] * 1.01,
        }
