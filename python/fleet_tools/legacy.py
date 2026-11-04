"""Legacy telemetry DB: seed history and simulate ongoing legacy-app activity."""
import argparse
import random
import sys
import time
from datetime import datetime, timedelta, timezone

import psycopg

from .backfill import default_end

PSI_PER_BAR = 14.5038
# 设备码: (rpm, coolant °C, hydraulic bar, fuel %/小时)
EQUIP = {
    "L1": (1400, 85, 200, 0.4),
    "L2": (1380, 84, 195, 0.4),
    "T1": (1800, 88, 150, 0.6),
    "T2": (1780, 87, 148, 0.6),
    "B1": (1200, 80, 250, 0.3),
}
FAULTS = ["E101", "E205", "E310", "E412"]
DSN = "postgresql://postgres:dev@localhost:5434/legacy"
INSERT = ("insert into legacy.telemetry_log "
          "(equip_code, recorded_at, rpm, coolant_c, hyd_psi, fuel_pct, hours, fault) "
          "values (%s,%s,%s,%s,%s,%s,%s,%s)")


def _clamp(x, lo, hi):
    return max(lo, min(hi, x))


def gen_rows(days=14, step_min=5, end=None, seed=7) -> list:
    end = end or (default_end() - timedelta(days=8))
    start = end - timedelta(days=days)
    rng = random.Random(seed)
    rows = []
    for code, (rpm0, cool0, hyd0, fuel_rate) in EQUIP.items():
        rpm, cool, hyd, fuel = float(rpm0), float(cool0), float(hyd0), 100.0
        hours = float(rng.randint(1000, 9000))
        t = start
        while t < end:
            running = 6 <= t.hour < 22
            if running:
                rpm = _clamp(rpm + rng.uniform(-1, 1) * 60, 600, 2800)
                cool = _clamp(cool + rng.uniform(-1, 1) * 0.5, 60, 110)
                hyd = _clamp(hyd + rng.uniform(-1, 1) * 8, 50, 340)
                fuel -= fuel_rate * step_min / 60
                if fuel < 5:
                    fuel = 100.0
                hours += step_min / 60
            else:
                rpm = 0.0
            fault = rng.choice(FAULTS) if running and rng.random() < 0.0005 else None
            rows.append((code, t, int(rpm), round(cool, 2), round(hyd * PSI_PER_BAR, 2),
                         round(fuel, 2), round(hours, 3), fault))
            t += timedelta(minutes=step_min)
    return rows


def seed(dsn, rows, chunk=1000) -> int:
    with psycopg.connect(dsn, autocommit=True) as conn:
        if conn.execute("select count(*) from legacy.telemetry_log").fetchone()[0]:
            raise SystemExit("legacy.telemetry_log is not empty; truncate it first to re-seed")
        for i in range(0, len(rows), chunk):
            with conn.transaction():          # 每块一个事务 => CDC 里是一个事务
                with conn.cursor() as cur:
                    cur.executemany(INSERT, rows[i:i + chunk])
        return conn.execute("select count(*) from legacy.telemetry_log").fetchone()[0]


def churn(dsn, rounds, interval, seed_) -> None:
    rng = random.Random(seed_)
    with psycopg.connect(dsn, autocommit=True) as conn:
        for r in range(rounds):
            now = datetime.now(timezone.utc).replace(microsecond=0)
            with conn.transaction():
                for code in rng.sample(list(EQUIP), 3):
                    rpm0, cool0, hyd0, _ = EQUIP[code]
                    conn.execute(INSERT, (code, now, rpm0 + rng.randint(-50, 50), cool0,
                                          round(hyd0 * PSI_PER_BAR, 2), 80.0, 9999.0, None))
                upd = conn.execute("select id from legacy.telemetry_log order by random() limit 1").fetchone()[0]
                conn.execute("update legacy.telemetry_log set rpm = coalesce(rpm, 0) + 50 where id = %s", (upd,))
                dele = None
                if rng.random() < 0.3:
                    dele = conn.execute("select id from legacy.telemetry_log where id <> %s "
                                        "order by random() limit 1", (upd,)).fetchone()[0]
                    conn.execute("delete from legacy.telemetry_log where id = %s", (dele,))
            print(f"round {r + 1}/{rounds}: +3 inserts, updated id={upd}, deleted id={dele}")
            if r < rounds - 1:
                time.sleep(interval)


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--dsn", default=DSN)
    sub = ap.add_subparsers(dest="cmd", required=True)
    s = sub.add_parser("seed")
    s.add_argument("--days", type=int, default=14)
    c = sub.add_parser("churn")
    c.add_argument("--rounds", type=int, default=5)
    c.add_argument("--interval", type=float, default=2.0)
    c.add_argument("--seed", type=int, default=1)
    args = ap.parse_args(argv)

    if args.cmd == "seed":
        rows = gen_rows(days=args.days)
        print(f"generated {len(rows)} rows; table now has {seed(args.dsn, rows)} rows")
    else:
        churn(args.dsn, args.rounds, args.interval, args.seed)
    return 0


if __name__ == "__main__":
    sys.exit(main())
