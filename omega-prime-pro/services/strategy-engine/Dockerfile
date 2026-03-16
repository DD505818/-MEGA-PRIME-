FROM python:3.11-slim
WORKDIR /app
COPY . /app
RUN pip install --no-cache-dir fastapi uvicorn pydantic ccxt numpy pandas
CMD ["python", "app.py"]
