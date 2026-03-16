from fastapi import FastAPI
app = FastAPI(title="Control Panel")

@app.get('/actions')
def actions():
    return {
      "actions": ["start/stop strategies","change risk parameters","deploy AI models","view strategy PnL","restart services"]
    }
