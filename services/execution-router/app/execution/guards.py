from .models import Order


class Guard:
    def __init__(self, max_notional: float, max_slippage_bps: float):
        self.max_notional = max_notional
        self.max_slippage_bps = max_slippage_bps

    def validate(self, order: Order, expected_slippage_bps: float, ref_price: float) -> None:
        notional = abs(order.qty * ref_price)
        if notional > self.max_notional:
            raise ValueError(f"max_notional_exceeded: {notional:.2f} > {self.max_notional:.2f}")
        if expected_slippage_bps > self.max_slippage_bps:
            raise ValueError(f"max_slippage_exceeded: {expected_slippage_bps:.2f}bps > {self.max_slippage_bps:.2f}bps")
