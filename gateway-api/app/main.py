from fastapi import FastAPI

app = FastAPI(title="omega-prime-gateway")

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/routes")
def routes():
    return {
        "services": [
            "market-data-service", "feature-engine", "strategy-engine", "rl-agent-cluster",
            "execution-router", "risk-engine", "portfolio-service", "backtesting-cluster"
        ]
    }
