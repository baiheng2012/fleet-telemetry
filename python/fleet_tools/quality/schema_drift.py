"""Detect drift between the JSON actually landing in RAW and the vehicle semantics contract."""
from dataclasses import dataclass

TABLES = ["raw.telemetry_events", "raw.telemetry_history"]

OBSERVED_SQL = """
select k as key, jsonb_typeof(payload -> k) as jtype, count(*) as n
from {table}, jsonb_object_keys(payload) as k
group by 1, 2
"""

ENVELOPE = {"vehicle_id": "string", "vehicle_type": "string", "ts": "string", "fault_code": "string"}
OPTIONAL = {"fault_code"}
JSON_TYPES = {"float": "number", "int": "number", "string": "string"}


@dataclass(frozen=True)
class Finding:
    kind: str       # unexpected_field | type_mismatch | missing_field
    key: str
    rows: int
    status: str     # warn | fail
    detail: str


def expected_schema(contract: dict) -> dict:
    exp = dict(ENVELOPE)
    for name, s in contract["signals"].items():
        exp[name] = JSON_TYPES[s["type"]]
    return exp


def classify(observed: dict, expected: dict) -> list:
    """observed: {(key, json_type): row_count}"""
    findings, seen = [], set()
    for (key, jtype), n in sorted(observed.items()):
        seen.add(key)
        if key not in expected:
            findings.append(Finding("unexpected_field", key, n, "warn", f"not in contract (type {jtype})"))
        elif jtype != "null" and jtype != expected[key]:
            findings.append(Finding("type_mismatch", key, n, "fail", f"expected {expected[key]}, got {jtype}"))
    if observed:  # 空表不报"缺字段"
        for key in sorted(expected):
            if key not in OPTIONAL and key not in seen:
                findings.append(Finding("missing_field", key, 0, "fail", "required by contract but never seen"))
    return findings
