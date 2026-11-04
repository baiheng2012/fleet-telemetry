from datetime import datetime, timezone

from fleet_tools import legacy

END = datetime(2026, 1, 1, tzinfo=timezone.utc)


def test_gen_rows_deterministic():
    a = legacy.gen_rows(days=1, end=END, seed=3)
    assert a == legacy.gen_rows(days=1, end=END, seed=3)
    assert len(a) == 5 * 24 * 12  # 5 台设备 × 24h × 每小时12个样本


def test_psi_values_within_physical_range():
    rows = legacy.gen_rows(days=1, end=END)
    assert max(r[4] for r in rows) <= 350 * legacy.PSI_PER_BAR
