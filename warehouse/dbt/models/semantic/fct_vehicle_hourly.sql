select
    vehicle_id,
    date_trunc('hour', ts at time zone 'UTC')                as hour_start_utc,
    count(*)                                                 as samples,
    avg(engine_rpm)                                          as avg_engine_rpm,
    max(coolant_temp)                                        as max_coolant_temp,
    avg(hydraulic_pressure)                                  as avg_hydraulic_pressure,
    min(fuel_level)                                          as min_fuel_level,
    max(hour_meter) - min(hour_meter)                        as engine_hours,
    count(*) filter (where fault_code is not null)           as fault_samples,
    count(*) filter (where cardinality(dq_flags) > 0)        as dq_flagged_samples
from {{ ref('curated_telemetry') }}
group by 1, 2
