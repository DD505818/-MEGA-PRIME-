class Strategy:
    def __init__(self, name, win_rate, max_risk):
        self.name = name
        self.win_rate = win_rate
        self.max_risk = max_risk

    def on_tick(self, tick, context):
        raise NotImplementedError
