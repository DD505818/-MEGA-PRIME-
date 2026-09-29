// Market Data Service — REAL Kraken spot data. No synthesis.
//
// Polls Kraken's public Ticker endpoint for XBT/USD and publishes:
//   market.raw    — full tick {exchange, symbol, price, bid, ask, volume,
//                   timestamp} (consumed by feature-engine)
//   market.prices — {symbol: price} map (consumed by portfolio-service for
//                   mark-to-market; 1B.4 made this topic real)
//
// The previous build published synthetic random-walk "Binance" ticks with
// timestamp: 0. That fabrication is gone: every field below comes from the
// exchange response, and timestamp is the local receipt time in Unix ms.
// If Kraken is unreachable or the response is malformed, nothing is
// published — fail-silent on data, never synthetic.

use rdkafka::config::ClientConfig;
use rdkafka::producer::{FutureProducer, FutureRecord, Producer};
use serde_json::json;
use std::env;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

const KRAKEN_TICKER_URL: &str = "https://api.kraken.com/0/public/Ticker?pair=XBTUSD";
const KRAKEN_PAIR_KEY: &str = "XXBTZUSD";
const SYMBOL: &str = "BTC/USD";
const POLL_INTERVAL: Duration = Duration::from_secs(5);

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

fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}

struct Tick {
    price: f64,
    bid: f64,
    ask: f64,
    volume: f64,
}

/// Fetch one real tick from Kraken's public API. Returns None on any
/// failure — the caller publishes nothing rather than fabricating data.
async fn fetch_kraken_tick(client: &reqwest::Client) -> Option<Tick> {
    let v: serde_json::Value = client.get(KRAKEN_TICKER_URL).send().await.ok()?.json().await.ok()?;
    parse_kraken_tick(&v)
}

/// Parse a Kraken Ticker response into a Tick. Pure function — unit-tested
/// below, because Kraken's nested response shape is the fragile part.
fn parse_kraken_tick(v: &serde_json::Value) -> Option<Tick> {
    if !v["error"].as_array().map(|e| e.is_empty()).unwrap_or(false) {
        eprintln!("kraken returned errors: {}", v["error"]);
        return None;
    }
    let pair = &v["result"][KRAKEN_PAIR_KEY];
    Some(Tick {
        price: pair["c"][0].as_str().and_then(|s| s.parse::<f64>().ok())?,
        bid: pair["b"][0].as_str().and_then(|s| s.parse::<f64>().ok())?,
        ask: pair["a"][0].as_str().and_then(|s| s.parse::<f64>().ok())?,
        volume: pair["c"][1].as_str().and_then(|s| s.parse::<f64>().ok()).unwrap_or(0.0),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    const SAMPLE: &str = r#"{"error":[],"result":{"XXBTZUSD":{"a":["82999.00000","1","1.000"],"b":["82998.90000","1","1.000"],"c":["82998.90000","0.00603612"],"v":["268.40588255","2862.86180275"],"p":["83083.21647","83278.12284"],"t":[18192,153218],"l":["82735.10000","82566.40000"],"h":["83581.70000","84338.10000"],"o":"83462.70000"}}}"#;

    #[test]
    fn parses_real_kraken_shape() {
        let v: serde_json::Value = serde_json::from_str(SAMPLE).unwrap();
        let tick = parse_kraken_tick(&v).expect("must parse the documented Kraken shape");
        assert!((tick.price - 82998.9).abs() < 1e-9);
        assert!((tick.bid - 82998.9).abs() < 1e-9);
        assert!((tick.ask - 82999.0).abs() < 1e-9);
        assert!(tick.volume > 0.0);
    }

    #[test]
    fn rejects_kraken_errors_and_garbage() {
        let err: serde_json::Value = serde_json::from_str(r#"{"error":["EQuery:Unknown asset pair"],"result":{}}"#).unwrap();
        assert!(parse_kraken_tick(&err).is_none());
        let empty: serde_json::Value = serde_json::from_str(r#"{}"#).unwrap();
        assert!(parse_kraken_tick(&empty).is_none());
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

    let http = reqwest::Client::builder()
        .timeout(Duration::from_secs(10))
        .user_agent("omega-prime-market-data/1.0")
        .build()
        .expect("HTTP client build failed");

    loop {
        match fetch_kraken_tick(&http).await {
            Some(tick) => {
                let ts = now_ms();
                let raw = json!({
                    "exchange": "kraken",
                    "symbol": SYMBOL,
                    "price": tick.price,
                    "bid": tick.bid,
                    "ask": tick.ask,
                    "volume": tick.volume,
                    "timestamp": ts,
                });
                let raw_payload = raw.to_string();
                if let Err(e) = producer
                    .send(
                        FutureRecord::to("market.raw").payload(&raw_payload).key(SYMBOL),
                        Duration::from_secs(1),
                    )
                    .await
                {
                    eprintln!("market.raw publish failed: {e:?}");
                }

                let prices = json!({ SYMBOL: tick.price });
                let prices_payload = prices.to_string();
                if let Err(e) = producer
                    .send(
                        FutureRecord::to("market.prices").payload(&prices_payload).key(SYMBOL),
                        Duration::from_secs(1),
                    )
                    .await
                {
                    eprintln!("market.prices publish failed: {e:?}");
                }
            }
            None => eprintln!("kraken poll failed — publishing nothing (no synthetic fallback)"),
        }
        tokio::time::sleep(POLL_INTERVAL).await;
    }
}
