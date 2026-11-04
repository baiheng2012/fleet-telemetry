select
    stream_seq,
    payload->>'op'                        as op,
    (payload->>'lsn')::bigint             as lsn,
    (payload->>'seq')::int                as seq,
    (payload->>'commit_ts')::timestamptz  as commit_ts,
    case when payload->>'op' = 'd' then payload->'before' else payload->'after' end as row_data
from {{ source('raw', 'legacy_cdc_events') }}
where payload->>'table' = 'telemetry_log'
