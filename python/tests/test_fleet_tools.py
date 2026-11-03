from datetime import datetime, timezone

from fleet_tools import backfill, contract_codegen
from fleet_tools.contract import load_contract

C = load_contract()
END = datetime(2026, 1, 1, tzinfo=timezone.utc)


def test_backfill_deterministic():
    a = backfill.generate_rows(C, days=1, step_s=600, seed=1, end=END)
    b = backfill.generate_rows(C, days=1, step_s=600, seed=1, end=END)
    assert a == b and len(a) > 0


def test_backfill_covers_fleet():
    rows = backfill.generate_rows(C, days=1, step_s=600, seed=1, end=END)
    assert {r["vehicle_id"] for r in rows} == {
        "LHD-001", "LHD-002", "Truck-001", "Truck-002", "Bolter-001"}


def test_macros_cover_every_signal():
    sql = contract_codegen.gen_macros(C)
    for name in C["signals"]:
        assert f"payload->>'{name}'" in sql
        assert f"'{name}_out_of_range'" in sql


def test_generated_files_up_to_date():
    assert contract_codegen.main(["--check"]) == 0
