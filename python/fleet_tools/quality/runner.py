"""Run data-quality checks, freshness SLAs and schema-drift detection; persist to dq.check_results."""
import argparse
import json
import sys
import time
import uuid
from collections import Counter
from dataclasses import dataclass, field

import psycopg

from ..contract import DEFAULT_CONTRACT, load_contract
from . import schema_drift
from .checks import build_checks, evaluate


@dataclass
class Result:
    check_name: str
    scope: str
    value: float | None
    warn: float | None
    fail: float | None
    status: str
    details: dict = field(default_factory=dict)


def run_checks(conn, checks) -> list:
    out = []
    for chk in checks:
        try:
            rows = conn.execute(chk.sql).fetchall()
        except psycopg.Error as e:  # 检查本身坏了 = fail，不能静默
            out.append(Result(chk.name, "*", None, chk.warn, chk.fail, "fail", {"error": str(e).strip()}))
            continue
        if not rows:
            out.append(Result(chk.name, "*", None, chk.warn, chk.fail, "warn", {"detail": "no data"}))
        for scope, value in rows:
            v = None if value is None else float(value)
            out.append(Result(chk.name, str(scope), v, chk.warn, chk.fail,
                              evaluate(v, chk.warn, chk.fail), {"description": chk.description}))
    return out


def run_drift(conn, expected) -> list:
    out = []
    for table in schema_drift.TABLES:
        try:
            rows = conn.execute(schema_drift.OBSERVED_SQL.format(table=table)).fetchall()
        except psycopg.Error as e:
            out.append(Result("schema_drift", table, None, None, None, "fail", {"error": str(e).strip()}))
            continue
        findings = schema_drift.classify({(k, t): n for k, t, n in rows}, expected)
        if not findings:
            out.append(Result("schema_drift", table, 0.0, None, None, "pass"))
        for f in findings:
            out.append(Result(f"schema_drift_{f.kind}", f"{table}.{f.key}", float(f.rows),
                              None, None, f.status, {"detail": f.detail}))
    return out


def save(conn, run_id, results) -> None:
    sql = ("insert into dq.check_results "
           "(run_id, check_name, scope, value, warn_threshold, fail_threshold, status, details) "
           "values (%s, %s, %s, %s, %s, %s, %s, %s::jsonb)")
    with conn.transaction():
        with conn.cursor() as cur:
            cur.executemany(sql, [
                (run_id, r.check_name, r.scope, r.value, r.warn, r.fail, r.status, json.dumps(r.details))
                for r in results
            ])


def print_report(results, verbose: bool) -> None:
    order = {"fail": 0, "warn": 1, "pass": 2}
    for r in sorted(results, key=lambda r: (order[r.status], r.check_name, r.scope)):
        if r.status == "pass" and not verbose:
            continue
        v = "n/a" if r.value is None else f"{r.value:.6g}"
        extra = r.details.get("error") or r.details.get("detail") or ""
        print(f"{r.status.upper():5} {r.check_name:34} {r.scope:36} {v:>12}  {extra}")
    c = Counter(r.status for r in results)
    print(f"-- {c['pass']} pass, {c['warn']} warn, {c['fail']} fail")


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--dsn", default="postgresql://postgres:dev@localhost:5433/warehouse")
    ap.add_argument("--contract", default=str(DEFAULT_CONTRACT))
    ap.add_argument("--watch", type=int, default=0, help="repeat every N seconds")
    ap.add_argument("--strict", action="store_true", help="exit 1 if any check fails")
    ap.add_argument("-v", "--verbose", action="store_true", help="also print passing checks")
    args = ap.parse_args(argv)

    contract = load_contract(args.contract)
    checks = build_checks(contract)
    expected = schema_drift.expected_schema(contract)

    while True:
        with psycopg.connect(args.dsn, autocommit=True) as conn:
            results = run_checks(conn, checks) + run_drift(conn, expected)
            save(conn, uuid.uuid4(), results)
        print_report(results, args.verbose)
        any_fail = any(r.status == "fail" for r in results)
        if not args.watch:
            break
        print()
        time.sleep(args.watch)
    return 1 if (args.strict and any_fail) else 0


if __name__ == "__main__":
    sys.exit(main())
