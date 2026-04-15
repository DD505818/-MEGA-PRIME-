from pydantic import BaseModel, Field
from typing import Any, Optional
import time


class Order(BaseModel):
    symbol: str
    side: str
    qty: float
    order_type: str = "market"
    limit_price: Optional[float] = None
    strategy_id: Optional[str] = None
    client_order_id: Optional[str] = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class Fill(BaseModel):
    fill_id: str
    venue: str
    symbol: str
    side: str
    qty: float
    price: float
    ts_ms: int = Field(default_factory=lambda: int(time.time() * 1000))
