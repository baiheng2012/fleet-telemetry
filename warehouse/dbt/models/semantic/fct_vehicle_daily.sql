select
    vehicle_id,
    (ts at time zone 'UTC')::date                            as day,
    count(*)                                                 as samples,
    max(hour_meter) - min(hour_meter)                        as engine_hours,
    round(((max(hour_meter) - min(hour_meter)) / 24.0)::numeric, 4) as utilization_ratio,
    avg(engine_rpm)                                          as avg_engine_rpm,
    max(coolant_temp)                                        as max_coolant_temp,
    count(*) filter (where fault_code is not null)           as fault_samples,
    count(*) filter (where cardinality(dq_flags) > 0)        as dq_flagged_samples
from {{ ref('curated_telemetry') }}
group by 1, 2
