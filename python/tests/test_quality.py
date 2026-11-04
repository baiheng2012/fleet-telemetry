from fleet_tools import contract_diff
from fleet_tools.contract import load_contract
from fleet_tools.quality import schema_drift
from fleet_tools.quality.checks import build_checks, evaluate

C = load_contract()


def test_evaluate_thresholds():
    assert evaluate(0.0, 0.01, 0.05) == "pass"
    assert evaluate(0.02, 0.01, 0.05) == "warn"
    assert evaluate(0.10, 0.01, 0.05) == "fail"
    assert evaluate(None, 0.01, 0.05) == "warn"   # no data is never a silent pass


def test_checks_generated_for_every_signal():
    checks = {c.name: c for c in build_checks(C)}
    for name in C["signals"]:
        assert f"'{name}' as scope" in checks["null_rate"].sql
        assert f"'{name}' as scope" in checks["out_of_range_rate"].sql


def test_drift_classification():
    exp = schema_drift.expected_schema(C)
    observed = {(k, t): 10 for k, t in exp.items()}
    assert schema_drift.classify(observed, exp) == []

    observed[("gps_lat", "number")] = 3
    observed[("engine_rpm", "string")] = 2
    del observed[("coolant_temp", exp["coolant_temp"])]
    kinds = {(f.kind, f.key) for f in schema_drift.classify(observed, exp)}
    assert ("unexpected_field", "gps_lat") in kinds
    assert ("type_mismatch", "engine_rpm") in kinds
    assert ("missing_field", "coolant_temp") in kinds


def test_drift_empty_table_is_not_reported_missing():
    assert schema_drift.classify({}, schema_drift.expected_schema(C)) == []


def _contract(**over):
    base = {
        "version": 1,
        "signals": {"a": {"unit": "x", "type": "float", "range": [0, 10], "applies_to": ["LHD"]}},
        "fault_codes": ["E1"],
    }
    base.update(over)
    return base


def test_contract_diff_breaking_and_compatible():
    old = _contract()
    narrowed = _contract(version=2, signals={
        "a": {"unit": "x", "type": "float", "range": [0, 5], "applies_to": ["LHD"]}})
    assert any(l == "breaking" for l, _ in contract_diff.diff(old, narrowed))

    widened = _contract(version=2, signals={
        "a": {"unit": "x", "type": "float", "range": [0, 20], "applies_to": ["LHD", "Truck"]}})
    assert all(l == "compatible" for l, _ in contract_diff.diff(old, widened))


def test_contract_diff_requires_version_bump():
    old = _contract()
    new = _contract(fault_codes=["E1", "E2"])   # 兼容变更，但没升版本
    assert any("version not bumped" in m for _, m in contract_diff.diff(old, new))
    assert contract_diff.diff(old, old) == []
