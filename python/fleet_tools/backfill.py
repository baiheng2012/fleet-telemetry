"""Historical batch backfill into raw.telemetry_history."""
import argparse
import json
import random
import sys
from datetime import datetime, timedelta, timezone

from .contract import DEFAULT_CONTRACT, load_contract

FLEET = {"LHD": 2, "Truck": 2, "Bolter": 1}
PROFILES = {
    "LHD":    dict(rpm=1400, rpm_step=60, cool=85, cool_step=0.5, hyd=200, hyd_step=8,  fuel_rate=0.004),
    "Truck":  dict(rpm=1800, rpm_step=80, cool=88, cool_step=0.6, hyd=150, hyd_step=6,  fuel_rate=0.006),
    "Bolter": dict(rpm=1200, rpm_step=40, cool=80, cool_step=0.4, hyd=250, hyd_step=10, fuel_rate=0.003),
}
AMBIENT = 25.0
P_DUP, P_NULL, P_SPIKE, P_BAD_FAULT, P_FAULT = 0.005, 0.01, 0.002, 0.001, 0.0005


def _clamp(x, lo, hi):
    return max(lo, min(hi, x))


def default_end() -> datetime:
    """今天 00:00 UTC。历史数据止于此，避免和当天的流式数据混在同一天。"""
    return datetime.now(timezone.utc).replace(hour=0, minute=0, second=0, microsecond=0)


def _simulate(vid, vtype, contract, start, end, step_s, seed):
    rng = random.Random(f"{seed}:{vid}")  # 字符串种子，跨运行稳定
    p = PROFILES[vtype]
    rg = {n: tuple(s["range"]) for n, s in contract["signals"].items()}
    codes = contract.get("fault_codes", [])

    rpm, cool, hyd, fuel = p["rpm"], p["cool"], p["hyd"], 100.0
    hour_meter = float(rng.randint(0, 5000))
    fault, fault_left = None, 0
    running, cur_hour = False, None

    t = start
    while t < end:
        hk = (t.date(), t.hour)
        if hk != cur_hour:  # 每小时重新决定是否运行
            cur_hour, was = hk, running
            running = 6 <= t.hour < 22 and rng.random() > 0.10
            if running and not was:
                rpm, hyd = p["rpm"], p["hyd"]
                cool = p["cool"]

        if running:
            rpm = _clamp(rpm + rng.uniform(-1, 1) * p["rpm_step"], *rg["engine_rpm"])
            cool = _clamp(cool + rng.uniform(-1, 1) * p["cool_step"], *rg["coolant_temp"])
            hyd = _clamp(hyd + rng.uniform(-1, 1) * p["hyd_step"], *rg["hydraulic_pressure"])
            fuel -= p["fuel_rate"] * step_s
            if fuel < 5:
                fuel = 100.0
            hour_meter += step_s / 3600
            if fault_left > 0:
                fault_left -= 1
                if fault_left == 0:
                    fault = None
            elif codes and rng.random() < P_FAULT:
                fault, fault_left = rng.choice(codes), rng.randint(5, 15)
        else:  # 停机：转速、液压归零，水温向环境温度回落，小时表不走
            rpm, hyd = 0.0, 0.0
            cool += (AMBIENT - cool) * 0.05
            fault, fault_left = None, 0

        rec = {
            "vehicle_id": vid, "vehicle_type": vtype,
            "ts": t.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "engine_rpm": round(rpm, 3), "coolant_temp": round(cool, 3),
            "hydraulic_pressure": round(hyd, 3), "fuel_level": round(fuel, 3),
            "hour_meter": round(hour_meter, 6),
        }
        if fault:
            rec["fault_code"] = fault

        # 脏数据注入
        if rng.random() < P_NULL:
            rec.pop("coolant_temp")
        if rng.random() < P_SPIKE:
            name, val = rng.choice([("engine_rpm", -100.0), ("coolant_temp", 500.0)])
            rec[name] = val
        if rng.random() < P_BAD_FAULT:
            rec["fault_code"] = "E999"

        yield rec
        if rng.random() < P_DUP:
            yield dict(rec)
        t += timedelta(seconds=step_s)


def generate_rows(contract, days=7, step_s=60, seed=42, end=None, fleet=None) -> list:
    end = end or default_end()
    start = end - timedelta(days=days)
    rows = []
    for vtype, n in (fleet or FLEET).items():
        for i in range(1, n + 1):
            rows.extend(_simulate(f"{vtype}-{i:03d}", vtype, contract, start, end, step_s, seed))
    return rows


def load(dsn: str, batch_id: str, rows: list, chunk: int = 5000) -> int:
    import psycopg

    sql = ("INSERT INTO raw.telemetry_history (batch_id, row_num, payload) "
           "VALUES (%s, %s, %s::jsonb) ON CONFLICT DO NOTHING")
    with psycopg.connect(dsn) as conn:
        with conn.cursor() as cur:
            for i in range(0, len(rows), chunk):
                part = [(batch_id, i + j, json.dumps(r, separators=(",", ":")))
                        for j, r in enumerate(rows[i:i + chunk])]
                cur.executemany(sql, part)
            cur.execute("select count(*) from raw.telemetry_history where batch_id = %s", (batch_id,))
            return cur.fetchone()[0]


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--days", type=int, default=7)
    ap.add_argument("--step", type=int, default=60, help="sample interval, seconds")
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--contract", default=str(DEFAULT_CONTRACT))
    ap.add_argument("--dsn", default="postgresql://postgres:dev@localhost:5433/warehouse")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args(argv)

    contract = load_contract(args.contract)
    end = default_end()
    rows = generate_rows(contract, args.days, args.step, args.seed, end)
    batch_id = f"backfill-{end:%Y%m%d}-seed{args.seed}-{args.days}d"
    print(f"generated {len(rows)} rows, batch_id={batch_id}")
    if args.dry_run:
        return 0
    print(f"rows in table for this batch: {load(args.dsn, batch_id, rows)} (rerun is idempotent)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
