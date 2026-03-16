class PPOPolicy:
    name = "PPO"

    def train(self):
        return {"status": "trained", "algo": self.name}

class DQNPolicy:
    name = "DQN"

    def train(self):
        return {"status": "trained", "algo": self.name}
