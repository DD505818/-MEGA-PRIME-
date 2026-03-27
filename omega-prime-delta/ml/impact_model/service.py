from fastapi import FastAPI
app = FastAPI()
@app.post('/predict')
def predict(payload: dict):
    return {'slippage': 0.001}
