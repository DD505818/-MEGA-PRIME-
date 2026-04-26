class BaseAgent:
    def __init__(self, name):
        self.name = name
    def generate_signal(self, context):
        raise NotImplementedError
    def risk_profile(self):
        return {}
