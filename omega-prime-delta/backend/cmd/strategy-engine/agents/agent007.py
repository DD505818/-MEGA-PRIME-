from .base import Strategy


class Agent007(Strategy):
    def __init__(self):
        super().__init__("Agent007", 0.80, 0.05)

    def on_tick(self, tick, context):
        return None
