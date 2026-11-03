-- GENERATED from contracts/vehicle_semantics.yaml. DO NOT EDIT.
-- dbt singular test: returns violating rows; passes when empty.
select vehicle_id, ts, 'engine_rpm' as signal, engine_rpm::double precision as value
from {{ ref('curated_telemetry') }}
where engine_rpm is not null and (engine_rpm < 0 or engine_rpm > 3000)
union all
select vehicle_id, ts, 'coolant_temp' as signal, coolant_temp::double precision as value
from {{ ref('curated_telemetry') }}
where coolant_temp is not null and (coolant_temp < -20 or coolant_temp > 130)
union all
select vehicle_id, ts, 'hydraulic_pressure' as signal, hydraulic_pressure::double precision as value
from {{ ref('curated_telemetry') }}
where hydraulic_pressure is not null and (hydraulic_pressure < 0 or hydraulic_pressure > 350)
union all
select vehicle_id, ts, 'fuel_level' as signal, fuel_level::double precision as value
from {{ ref('curated_telemetry') }}
where fuel_level is not null and (fuel_level < 0 or fuel_level > 100)
union all
select vehicle_id, ts, 'hour_meter' as signal, hour_meter::double precision as value
from {{ ref('curated_telemetry') }}
where hour_meter is not null and (hour_meter < 0 or hour_meter > 100000)
