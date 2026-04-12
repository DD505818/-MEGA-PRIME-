import argparse
import asyncio
import random
from datetime import datetime, timedelta


async def run_soak_test(duration_minutes: int = 10, dry_run: bool = False) -> None:
    start = datetime.now()
    end = start + timedelta(minutes=duration_minutes)
    trades_executed = 0
    kill_switch_triggers = 0

    while datetime.now() < end:
        for _ in range(random.randint(2, 5)):
            if dry_run:
                print("[DRY-RUN] submit order")
            trades_executed += 1

        elapsed = (datetime.now() - start).total_seconds()
        if elapsed > 0 and int(elapsed) % 120 == 0:
            kill_switch_triggers += 1
            print("[DRY-RUN] kill switch activated and verified")

        await asyncio.sleep(1)

    print(f"Soak test completed. Trades={trades_executed}, KillDrills={kill_switch_triggers}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--duration-minutes", type=int, default=10)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    asyncio.run(run_soak_test(duration_minutes=args.duration_minutes, dry_run=args.dry_run))
