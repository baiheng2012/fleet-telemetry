"""Rolling-window features and robust anomaly detection on curated telemetry."""
import argparse
import sys

import pandas as pd
import psycopg

BUCKET_SQL = """
select vehicle_id, min(vehicle_type) as vehicle_type,
       to_timestamp(floor(extract(epoch from ts) / 300) * 300) as bucket,
       avg(engine_rpm) as rpm_mean, avg(coolant_temp) as coolant_mean,
       avg(hydraulic_pressure) as hyd_mean, min(fuel_level) as fuel_min,
       (count(*) filter (where fault_code is not null))::int as fault_count
from curated.curated_telemetry
group by 1, 3
order by 1, 3
"""

FEATURE_COLS = [
    "vehicle_id", "bucket", "vehicle_type", "running", "rpm_mean", "coolant_mean", "hyd_mean",
    "fuel_min", "fault_count", "coolant_mean_30m", "coolant_std_30m", "hyd_mean_30m",
    "hyd_std_30m", "coolant_delta_30m",
]
ANOMALY_COLS = ["vehicle_id", "signal", "bucket", "value", "baseline", "z"]
SIGNALS = {"coolant_temp": "coolant_mean", "hydraulic_pressure": "hyd_mean"}
WINDOW = 6          # 6 个桶 = 30 分钟（假设桶连续，缺口处窗口会变长，可接受）
BASELINE = 24       # 24 个桶 = 2 小时滚动中位数


def build_features(df: pd.DataFrame) -> pd.DataFrame:
    df = df.sort_values(["vehicle_id", "bucket"]).reset_index(drop=True)
    df["running"] = df["rpm_mean"].fillna(0) > 100
    g = df.groupby("vehicle_id")
    for col, name in (("coolant_mean", "coolant"), ("hyd_mean", "hyd")):
        df[f"{name}_mean_30m"] = g[col].transform(lambda s: s.rolling(WINDOW, min_periods=3).mean())
        df[f"{name}_std_30m"] = g[col].transform(lambda s: s.rolling(WINDOW, min_periods=3).std())
    df["coolant_delta_30m"] = g["coolant_mean"].transform(lambda s: s - s.shift(WINDOW))
    return df


def detect_anomalies(df: pd.DataFrame, threshold: float = 8.0) -> pd.DataFrame:
    """偏离滚动中位数基线的程度，用 MAD 估计尺度（1.4826*MAD ≈ 正态下的标准差）。只看运行中的桶。"""
    rows = []
    for vid, g in df[df["running"]].groupby("vehicle_id"):
        for sig, col in SIGNALS.items():
            s = g.set_index("bucket")[col].dropna()
            if len(s) < BASELINE:
                continue
            base = s.rolling(BASELINE, min_periods=BASELINE // 2).median().shift(1)  # 基线不含当前点
            resid = (s - base).dropna()
            mad = (resid - resid.median()).abs().median()
            scale = max(1.4826 * mad, 0.01 * abs(s.median()), 1e-6)   # 下限防止常量信号误报
            z = resid / scale
            for bucket, zv in z[z.abs() > threshold].items():
                rows.append((vid, sig, bucket, float(s[bucket]), float(base[bucket]), float(zv)))
    return pd.DataFrame(rows, columns=ANOMALY_COLS)


def _py(v):
    if v is None or (not isinstance(v, (str, bool)) and pd.isna(v)):
        return None
    if isinstance(v, pd.Timestamp):
        return v.to_pydatetime()
    return v.item() if hasattr(v, "item") else v


def _insert(cur, table, cols, df):
    ph = ", ".join(["%s"] * len(cols))
    sql = f"insert into {table} ({', '.join(cols)}) values ({ph})"
    cur.executemany(sql, [tuple(_py(v) for v in r) for r in df[cols].itertuples(index=False, name=None)])


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--dsn", default="postgresql://postgres:dev@localhost:5433/warehouse")
    ap.add_argument("--threshold", type=float, default=8.0)
    args = ap.parse_args(argv)

    with psycopg.connect(args.dsn) as conn:
        cur = conn.execute(BUCKET_SQL)
        raw = pd.DataFrame(cur.fetchall(), columns=[d.name for d in cur.description])
        if raw.empty:
            print("no curated data; run dbt build first")
            return 1
        feats = build_features(raw)
        anoms = detect_anomalies(feats, args.threshold)
        with conn.transaction():
            conn.execute("delete from semantic.ml_features")
            conn.execute("delete from semantic.anomalies")
            with conn.cursor() as c:
                _insert(c, "semantic.ml_features", FEATURE_COLS, feats)
                _insert(c, "semantic.anomalies", ANOMALY_COLS, anoms)
    print(f"features: {len(feats)} rows, anomalies: {len(anoms)}")
    if len(anoms):
        print(anoms.sort_values("z", key=abs, ascending=False).head(10).to_string(index=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
