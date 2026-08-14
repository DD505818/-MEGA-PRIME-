use rdkafka::config::ClientConfig;
use rdkafka::producer::{FutureProducer, FutureRecord, Producer};
use serde_json::json;
use std::env;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::Duration;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

async fn serve_health(ready: Arc<AtomicBool>) -> std::io::Result<()> {
    let port = env::var("HEALTH_PORT").unwrap_or_else(|_| "8090".to_string());
    let listener = TcpListener::bind(format!("0.0.0.0:{port}")).await?;
    loop {
        let (mut stream, _) = listener.accept().await?;
        let ready = Arc::clone(&ready);
        tokio::spawn(async move {
            let mut request = [0_u8; 1024];
            let count = stream.read(&mut request).await.unwrap_or(0);
            let request = String::from_utf8_lossy(&request[..count]);
            let path = request.split_whitespace().nth(1).unwrap_or("");
            let (status, body) = match path {
                "/health/live" | "/health" => ("200 OK", r#"{"status":"live"}"#),
                "/health/ready" if ready.load(Ordering::Relaxed) => {
                    ("200 OK", r#"{"status":"ready"}"#)
                }
                "/health/ready" => (
                    "503 Service Unavailable",
                    r#"{"status":"not_ready","dependency":"kafka"}"#,
                ),
                _ => ("404 Not Found", r#"{"status":"not_found"}"#),
            };
            let response = format!(
                "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
            let _ = stream.write_all(response.as_bytes()).await;
        });
    }
}

#[tokio::main]
async fn main() {
    let ready = Arc::new(AtomicBool::new(false));
    let health_ready = Arc::clone(&ready);
    tokio::spawn(async move {
        if let Err(error) = serve_health(health_ready).await {
            eprintln!("health server failed: {error}");
        }
    });

    let mut config = ClientConfig::new();
    config.set(
        "bootstrap.servers",
        env::var("KAFKA_BROKERS").unwrap_or_else(|_| "kafka:9092".to_string()),
    );
    for (env_key, kafka_key) in [
        ("KAFKA_SECURITY_PROTOCOL", "security.protocol"),
        ("KAFKA_SASL_MECHANISM", "sasl.mechanism"),
        ("KAFKA_SASL_USERNAME", "sasl.username"),
        ("KAFKA_SASL_PASSWORD", "sasl.password"),
        ("KAFKA_SSL_CA_LOCATION", "ssl.ca.location"),
    ] {
        if let Ok(value) = env::var(env_key) {
            config.set(kafka_key, value);
        }
    }
    let producer: FutureProducer = config.create().expect("Producer creation error");
    producer
        .client()
        .fetch_metadata(None, Duration::from_secs(5))
        .expect("Kafka metadata unavailable");
    ready.store(true, Ordering::Relaxed);

    let symbols = vec!["BTCUSDT", "ETHUSDT", "SOLUSDT"];
    let mut counter = 0u64;
    loop {
        for symbol in &symbols {
            let tick = json!({
                "exchange": "binance",
                "symbol": symbol,
                "price": 63000.0 + counter as f64,
                "volume": 1.5,
                "timestamp": 0,
                "bid": 62999.0,
                "ask": 63001.0
            });
            producer
                .send(
                    FutureRecord::to("market.raw")
                        .payload(&tick.to_string())
                        .key(&symbol),
                    Duration::from_secs(1),
                )
                .await
                .expect("Kafka publish failed");
        }
        counter += 1;
        tokio::time::sleep(Duration::from_millis(500)).await;
    }
}
