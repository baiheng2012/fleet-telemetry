select
    'stream'::text                  as source,
    stream_seq::text                as source_id,
    payload->>'vehicle_id'          as vehicle_id,
    payload->>'vehicle_type'        as vehicle_type,
    (payload->>'ts')::timestamptz   as ts,
    {{ contract_extract_signals() }},
    payload->>'fault_code'          as fault_code,
    ingested_at
from {{ source('raw', 'telemetry_events') }}
