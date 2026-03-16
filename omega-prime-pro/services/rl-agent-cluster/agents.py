class PPOAgent:
    def act(self, state):
        return {"policy": "ppo", "action": "hold", "confidence": 0.55}

class DQNAgent:
    def act(self, state):
        return {"policy": "dqn", "action": "buy", "confidence": 0.62}
