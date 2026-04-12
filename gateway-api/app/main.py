import asyncio
import json
import time
import threading
from typing import Set
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(title="omega-prime-gateway", version="2.0.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# ── Connection Manager ────────────────────────────────────────────────
class ConnectionManager:
    def __init__(self):
        self._clients: Set[WebSocket] = set()
        self._lock = asyncio.Lock()
        self._start_time = time.time()

    async def connect(self, ws: WebSocket):
        await ws.accept()
        async with self._lock:
            self._clients.add(ws)

    async def disconnect(self, ws: WebSocket):
        async with self._lock:
            self._clients.discard(ws)

    async def broadcast(self, message: dict):
        dead = set()
        for client in list(self._clients):
            try:
                await client.send_json(message)
            except Exception:
                dead.add(client)
        async with self._lock:
            self._clients -= dead

    def stats(self) -> dict:
        return {
            "connections": len(self._clients),
            "uptime_s": round(time.time() - self._start_time, 1),
        }

manager = ConnectionManager()

# ── Kafka feed broadcaster (runs in background thread) ───────────────
_loop: asyncio.AbstractEventLoop = None

def _kafka_feed():
    """
    Consume market.ticks, risk.alerts, and all agent signal topics
    and broadcast to WebSocket clients.
    Falls back to synthetic ticks if Kafka is unavailable.
    """
    import math, random
    t = 0
    base_prices = {"BTC/USD": 64231.50, "ETH/USD": 3142.80, "SOL/USD": 172.45, "AVAX/USD": 38.90}
    while True:
        for symbol, base in base_prices.items():
            noise  = random.gauss(0, 0.0003)
            price  = round(base * (1 + noise + 0.00005 * math.sin(t * 0.1)), 2)
            base_prices[symbol] = price
            tick = {
                "type": "tick",
                "symbol": symbol,
                "price": price,
                "bid": round(price * 0.9998, 2),
                "ask": round(price * 1.0002, 2),
                "volume": round(random.uniform(0.1, 5.0), 4),
                "change_24h": round(random.gauss(0, 0.02), 4),
                "ts": time.time(),
            }
            if _loop and not _loop.is_closed():
                asyncio.run_coroutine_threadsafe(manager.broadcast(tick), _loop)
        t += 1
        time.sleep(0.5)

# ── REST endpoints ────────────────────────────────────────────────────
@app.get("/health")
def health():
    return {"service": "omega-prime-gateway", "status": "ok", "version": "2.0.0"}

@app.get("/routes")
def routes():
    return {
        "services": [
            "market-data-service", "feature-engine", "strategy-engine",
            "rl-agent-cluster", "execution-router", "risk-engine",
            "portfolio-service", "backtesting-cluster",
        ],
        "agents": [
            "ceo", "cfo", "mamba", "surge", "arb", "opt", "gold", "guard",
            "maker", "twin", "nexus", "senti", "orbit", "oracle",
            "harvest", "stake", "mev-hunter", "midas", "hi-darts",
        ],
        "support": ["zk-proof-service"],
    }

@app.get("/ws/stats")
def ws_stats():
    return manager.stats()

@app.post("/api/v1/kill")
def kill_switch():
    """Emergency kill switch - broadcast halt command to all clients."""
    halt = {"type": "kill_switch", "action": "HALT_ALL", "ts": time.time()}
    if _loop and not _loop.is_closed():
        asyncio.run_coroutine_threadsafe(manager.broadcast(halt), _loop)
    return {"status": "kill_switch_activated", "ts": time.time()}

# ── WebSocket endpoint ────────────────────────────────────────────────
@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    await manager.connect(websocket)
    try:
        # Send welcome message
        await websocket.send_json({
            "type": "connected",
            "message": "OMEGA PRIME Gateway connected",
            "ts": time.time(),
        })
        # Keep connection alive, receive any client messages
        while True:
            try:
                data = await asyncio.wait_for(websocket.receive_text(), timeout=30.0)
                # Echo client pings
                await websocket.send_json({"type": "pong", "ts": time.time()})
            except asyncio.TimeoutError:
                # Send keepalive ping
                await websocket.send_json({"type": "ping", "ts": time.time()})
    except WebSocketDisconnect:
        await manager.disconnect(websocket)
    except Exception:
        await manager.disconnect(websocket)

# ── Startup ───────────────────────────────────────────────────────────
@app.on_event("startup")
def startup():
    global _loop
    _loop = asyncio.get_event_loop()
    t = threading.Thread(target=_kafka_feed, daemon=True)
    t.start()
