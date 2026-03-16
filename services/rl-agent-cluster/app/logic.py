class PPOAgent:
    def act(self, obs):
        return {"action": "buy", "confidence": 0.61}

class DQNAgent:
    def act(self, obs):
        return {"action": "sell", "confidence": 0.58}
