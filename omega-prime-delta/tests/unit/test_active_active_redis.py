from unittest.mock import patch

from backend.shared.utils.active_active_redis import ActiveActiveRedis


class FakeRedis:
    def __init__(self):
        self.store = {}

    def json(self):
        raise RuntimeError("json module unavailable")

    def set(self, key, value):
        self.store[key] = value

    def get(self, key):
        return self.store.get(key)


@patch("backend.shared.utils.active_active_redis.redis.Redis.from_url")
def test_active_active_set_and_get(mock_from_url):
    mock_from_url.side_effect = [FakeRedis(), FakeRedis(), FakeRedis()]

    client = ActiveActiveRedis(
        {
            "us-east": "redis://us",
            "eu-west": "redis://eu",
            "ap-southeast": "redis://ap",
        },
        local_region="us-east",
    )

    client.set("position:BTCUSD", {"qty": 1.2, "entry": 42000})
    assert client.get("position:BTCUSD") == {"qty": 1.2, "entry": 42000}
