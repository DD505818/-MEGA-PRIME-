import argparse
import asyncio
import json
import time

async def inject_stale_prices(iterations: int, dry_run: bool) -> None:
    if dry_run:
        for _ in range(iterations):
            msg = {
                "symbol": "BTCUSD",
                "price": 42000,
                "timestamp": time.time(),
            }
            print(f"[DRY-RUN] would send: {json.dumps(msg)}")
            await asyncio.sleep(0.01)
        return

    from kafka import KafkaProducer

    producer = KafkaProducer(
        bootstrap_servers="kafka:9092",
        value_serializer=lambda v: json.dumps(v).encode(),
    )
    for _ in range(iterations):
        producer.send(
            "market.ticks",
            {"symbol": "BTCUSD", "price": 42000, "timestamp": time.time()},
        )
        await asyncio.sleep(1)


async def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--iterations", type=int, default=60)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    print("Injecting stale BTC market data...")
    await inject_stale_prices(args.iterations, args.dry_run)
    print("Stale data injection complete")


if __name__ == "__main__":
    asyncio.run(main())
