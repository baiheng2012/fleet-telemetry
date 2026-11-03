select
    vehicle_id,
    min(vehicle_type) as vehicle_type,
    min(ts)           as first_seen,
    max(ts)           as last_seen,
    count(*)          as sample_count
from {{ ref('curated_telemetry') }}
group by vehicle_id
