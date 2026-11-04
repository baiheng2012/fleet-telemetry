"""Data-quality and freshness checks. Every check is a SQL query returning (scope, value)."""
from dataclasses import dataclass

CUR = "curated.curated_telemetry"


@dataclass(frozen=True)
class Check:
    name: str
    sql: str            # 必须返回两列：scope(text), value(float8)
    warn: float         # value > warn -> warn
    fail: float         # value > fail -> fail
    description: str = ""


def evaluate(value, warn, fail) -> str:
    """没有数据（None）判 warn，绝不静默通过。"""
    if value is None:
        return "warn"
    if value > fail:
        return "fail"
    if value > warn:
        return "warn"
    return "pass"


_NULL_T = (
    "select '{n}' as scope, "
    "(count(*) filter (where {n} is null))::float8 / nullif(count(*), 0) as value "
    "from {cur}"
)
_OOR_T = (
    "select '{n}' as scope, "
    "(count(*) filter (where '{n}_out_of_range' = any(dq_flags)))::float8 / nullif(count(*), 0) as value "
    "from {cur}"
)


def _per_signal(contract: dict, template: str) -> str:
    return "\nunion all\n".join(template.format(n=n, cur=CUR) for n in contract["signals"])


UNKNOWN_FAULT_SQL = f"""
select 'fault_code' as scope,
       (count(*) filter (where 'unknown_fault_code' = any(dq_flags)))::float8 / nullif(count(*), 0) as value
from {CUR}
"""

# 去重掉的行占 staging 行数的比例（按来源）。staging 是视图，实时反映 RAW。
DUPLICATE_SQL = f"""
select s.source as scope,
       (s.n - coalesce(c.n, 0))::float8 / nullif(s.n, 0) as value
from (
    select source, count(*) as n from (
        select source from staging.stg_stream_events
        union all
        select source from staging.stg_history_events
    ) u group by source
) s
left join (select source, count(*) as n from {CUR} group by source) c using (source)
"""

# 流式数据按到达顺序（stream_seq）看，同一辆车 ts 倒退的比例
OUT_OF_ORDER_SQL = """
select 'stream' as scope,
       (count(*) filter (where ts < prev_ts))::float8 / nullif(count(*), 0) as value
from (
    select ts, lag(ts) over (partition by vehicle_id order by source_id::bigint) as prev_ts
    from staging.stg_stream_events
    where vehicle_id is not null
) t
"""

# 小时表必须单调不减：按时间排序后每次下降计一次
HOUR_METER_DECREASE_SQL = f"""
select vehicle_id as scope,
       (count(*) filter (where hour_meter < prev_hm))::float8 as value
from (
    select vehicle_id, hour_meter,
           lag(hour_meter) over (partition by vehicle_id order by ts) as prev_hm
    from {CUR}
    where hour_meter is not null
) t
group by vehicle_id
"""

# 小时表增量不应超过实际经过的时间（允许 5% 误差）。违规的"车辆日"占比。
HOUR_METER_SPAN_SQL = f"""
select 'all' as scope,
       (count(*) filter (where hm > span_h * 1.05 + 0.01))::float8 / nullif(count(*), 0) as value
from (
    select vehicle_id,
           (ts at time zone 'UTC')::date as day,
           max(hour_meter) - min(hour_meter) as hm,
           extract(epoch from max(ts) - min(ts)) / 3600.0 as span_h
    from {CUR}
    group by 1, 2
    having count(*) > 1
) d
"""

# ---- 新鲜度 SLA：直接查 RAW，不依赖 dbt 是否刚跑过 ----

FRESHNESS_SQL = """
select payload->>'vehicle_id' as scope,
       extract(epoch from now() - max((payload->>'ts')::timestamptz))::float8 as value
from raw.telemetry_events
where published_at > now() - interval '7 days'
  and payload->>'vehicle_id' is not null
group by 1
"""

LATENCY_SQL = """
select 'stream' as scope,
       (percentile_cont(0.95) within group (
           order by extract(epoch from ingested_at - (payload->>'ts')::timestamptz)
       ))::float8 as value
from raw.telemetry_events
where ingested_at > now() - interval '1 hour'
"""

# curated 比 RAW 落后多少秒（dbt 没跑就会变大）
CURATED_LAG_SQL = f"""
select 'curated_vs_raw' as scope,
       extract(epoch from (
           (select max((payload->>'ts')::timestamptz) from raw.telemetry_events
             where published_at > now() - interval '7 days')
         - (select max(ts) from {CUR} where source = 'stream')
       ))::float8 as value
"""


def build_checks(contract: dict) -> list[Check]:
    return [
        Check("null_rate", _per_signal(contract, _NULL_T), 0.03, 0.10,
              "share of NULL values per signal (after cleaning)"),
        Check("out_of_range_rate", _per_signal(contract, _OOR_T), 0.005, 0.02,
              "share of rows flagged out_of_range per signal"),
        Check("unknown_fault_code_rate", UNKNOWN_FAULT_SQL, 0.001, 0.01,
              "share of rows with a fault code not in the contract"),
        Check("duplicate_rate", DUPLICATE_SQL, 0.01, 0.05,
              "share of staging rows removed by dedup, per source"),
        Check("out_of_order_rate", OUT_OF_ORDER_SQL, 0.01, 0.05,
              "share of stream messages arriving after a newer one (same vehicle)"),
        Check("hour_meter_decreases", HOUR_METER_DECREASE_SQL, 0, 2,
              "number of times hour_meter went backwards, per vehicle"),
        Check("hour_meter_span_violation_rate", HOUR_METER_SPAN_SQL, 0, 0.2,
              "share of vehicle-days whose hour_meter delta exceeds elapsed time"),
        Check("freshness_seconds", FRESHNESS_SQL, 120, 600,
              "SLA: seconds since last message, per vehicle"),
        Check("ingest_latency_p95_seconds", LATENCY_SQL, 5, 30,
              "SLA: p95 of (ingested_at - event ts) over the last hour"),
        Check("curated_lag_seconds", CURATED_LAG_SQL, 900, 3600,
              "SLA: how far curated is behind RAW (dbt run cadence)"),
    ]
