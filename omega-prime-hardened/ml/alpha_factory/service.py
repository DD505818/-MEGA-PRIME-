"""Alpha factory service entrypoint."""

from __future__ import annotations

import logging
import os
import signal
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

LOGGER = logging.getLogger("alpha_factory")


def validate_env() -> dict[str, str]:
    model_registry = os.getenv("ALPHA_MODEL_REGISTRY")
    if not model_registry:
        raise RuntimeError("ALPHA_MODEL_REGISTRY is required")
    return {"model_registry": model_registry}


class AlphaRequest(BaseModel):
    symbol: str = Field(min_length=1)
    horizon: int = Field(default=5, ge=1, le=60)


class AlphaResponse(BaseModel):
    symbol: str
    score: float
    model_registry: str


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.config = validate_env()
    LOGGER.info("alpha_factory_starting", extra=app.state.config)
    yield
    LOGGER.info("alpha_factory_stopping")


app = FastAPI(title="alpha-factory", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/score", response_model=AlphaResponse)
def score(payload: AlphaRequest) -> AlphaResponse:
    if len(payload.symbol) > 20:
        raise HTTPException(status_code=400, detail="symbol too long")
    score_value = round(min(0.99, 0.1 + payload.horizon * 0.01), 4)
    return AlphaResponse(
        symbol=payload.symbol.upper(),
        score=score_value,
        model_registry=app.state.config["model_registry"],
    )


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    config = uvicorn.Config(app, host="0.0.0.0", port=int(os.getenv("ALPHA_FACTORY_PORT", "8100")), log_config=None)
    server = uvicorn.Server(config)

    def _handle_signal(signum, _frame):
        LOGGER.info("signal_received", extra={"signal": signum})
        server.should_exit = True

    signal.signal(signal.SIGTERM, _handle_signal)
    signal.signal(signal.SIGINT, _handle_signal)
    server.run()


if __name__ == "__main__":
    main()
