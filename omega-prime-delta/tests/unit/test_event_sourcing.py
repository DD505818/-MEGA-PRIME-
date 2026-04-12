from services.event_sourcing.consumer import PortfolioReconstructor
from services.event_sourcing.producer import EventProducer


class DummyTracker:
    def __init__(self):
        self.positions = {}

    def update_from_fill(self, fill):
        sym = fill["symbol"]
        self.positions[sym] = self.positions.get(sym, 0.0) + float(fill["qty"])


def test_event_producer_idempotency_and_replay():
    producer = EventProducer()
    event_id = "evt-1"
    payload = {"id": "o-1", "symbol": "BTCUSD", "qty": 1}

    assert producer.publish_order(payload, event_id=event_id) == event_id
    assert producer.publish_order(payload, event_id=event_id) is None

    tracker = DummyTracker()
    recon = PortfolioReconstructor(tracker)
    recon.set_local_events(
        [
            {"event_type": "ORDER_PLACED", "data": payload},
            {"event_type": "ORDER_FILLED", "data": {"symbol": "BTCUSD", "qty": 1.5}},
        ]
    )
    assert recon.rebuild_from_scratch() == 2
    assert tracker.positions["BTCUSD"] == 1.5
