from fastapi import FastAPI
app = FastAPI(title="OMEGA PRIME Gateway")

@app.get('/health')
def health():
    return {"status": "ok"}
