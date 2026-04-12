"""Price predictor service entrypoint."""

from __future__ import annotations

import logging
import os
import signal
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel, Field

LOGGER = logging.getLogger("price_predictor")


def validate_env() -> dict[str, int]:
    window = int(os.getenv("PRICE_PREDICTOR_WINDOW", "20"))
    if window < 2:
        raise RuntimeError("PRICE_PREDICTOR_WINDOW must be >= 2")
    return {"window": window}


class PredictRequest(BaseModel):
    prices: list[float] = Field(min_length=2)


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.cfg = validate_env()
    LOGGER.info("price_predictor_starting", extra=app.state.cfg)
    yield
    LOGGER.info("price_predictor_stopping")


app = FastAPI(title="price-predictor", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/predict")
def predict(payload: PredictRequest) -> dict[str, float]:
    window = app.state.cfg["window"]
    series = payload.prices[-window:]
    forecast = series[-1] + (series[-1] - series[0]) / max(1, len(series) - 1)
    return {"next_price": round(forecast, 6)}


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    config = uvicorn.Config(app, host="0.0.0.0", port=int(os.getenv("PRICE_PREDICTOR_PORT", "8103")), log_config=None)
    server = uvicorn.Server(config)
    signal.signal(signal.SIGTERM, lambda *_: setattr(server, "should_exit", True))
    signal.signal(signal.SIGINT, lambda *_: setattr(server, "should_exit", True))
    server.run()


if __name__ == "__main__":
    main()
