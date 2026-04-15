from dataclasses import dataclass
import time


@dataclass
class CircuitBreaker:
    threshold: int = 5
    half_open_seconds: int = 30
    failure_count: int = 0
    state: str = "CLOSED"
    opened_at: float = 0.0

    def can_execute(self) -> bool:
        if self.state == "CLOSED":
            return True
        if self.state == "OPEN" and (time.time() - self.opened_at) >= self.half_open_seconds:
            self.state = "HALF_OPEN"
            return True
        return self.state == "HALF_OPEN"

    def record_success(self) -> None:
        self.failure_count = 0
        self.state = "CLOSED"

    def record_failure(self) -> None:
        self.failure_count += 1
        if self.failure_count >= self.threshold:
            self.state = "OPEN"
            self.opened_at = time.time()
