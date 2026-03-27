"""Meta-controller service for strategy weight blending."""

from __future__ import annotations

import logging
import os
import signal
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel, Field

LOGGER = logging.getLogger("meta_controller")


def validate_env() -> dict[str, float]:
    temperature = float(os.getenv("META_CONTROLLER_TEMPERATURE", "1.0"))
    if temperature <= 0:
        raise RuntimeError("META_CONTROLLER_TEMPERATURE must be > 0")
    return {"temperature": temperature}


class BlendRequest(BaseModel):
    scores: dict[str, float] = Field(min_length=1)


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.cfg = validate_env()
    LOGGER.info("meta_controller_starting", extra=app.state.cfg)
    yield
    LOGGER.info("meta_controller_stopping")


app = FastAPI(title="meta-controller", lifespan=lifespan)


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/blend")
def blend(payload: BlendRequest) -> dict[str, dict[str, float]]:
    temp = app.state.cfg["temperature"]
    scaled = {name: max(0.0, value / temp) for name, value in payload.scores.items()}
    total = sum(scaled.values()) or 1.0
    return {"weights": {name: round(value / total, 6) for name, value in scaled.items()}}


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    config = uvicorn.Config(app, host="0.0.0.0", port=int(os.getenv("META_CONTROLLER_PORT", "8102")), log_config=None)
    server = uvicorn.Server(config)
    signal.signal(signal.SIGTERM, lambda *_: setattr(server, "should_exit", True))
    signal.signal(signal.SIGINT, lambda *_: setattr(server, "should_exit", True))
    server.run()


if __name__ == "__main__":
    main()
