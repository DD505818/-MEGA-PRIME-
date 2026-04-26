use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::config::ClientConfig;
use serde_json::json;
use std::time::Duration;

#[tokio::main]
async fn main() {
    let producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", "kafka:9092")
        .create()
        .expect("Producer creation error");

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
            producer.send(
                FutureRecord::to("market.raw").payload(&tick.to_string()).key(&symbol),
                Duration::from_secs(1),
            ).await.unwrap();
        }
        counter += 1;
        tokio::time::sleep(Duration::from_millis(500)).await;
    }
}
