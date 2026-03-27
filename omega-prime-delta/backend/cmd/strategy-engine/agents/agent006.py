from .base import Strategy

class Agent006(Strategy):
    def __init__(self):
        super().__init__("Agent006", 0.70, 0.05)

    def on_tick(self, tick, context):
        return None
