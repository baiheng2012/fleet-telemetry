{{ config(indexes=[{'columns': ['vehicle_id', 'ts'], 'unique': True}]) }}

with unioned as (
    select * from {{ ref('stg_stream_events') }}
    union all
    select * from {{ ref('stg_history_events') }}
),

ranked as (
    select
        *,
        row_number() over (
            partition by vehicle_id, ts
            order by case source when 'stream' then 0 else 1 end, ingested_at, source_id
        ) as rn
    from unioned
    where vehicle_id is not null and ts is not null
),

deduped as (
    select * from ranked where rn = 1
)

select
    vehicle_id,
    vehicle_type,
    ts,
    source,
    source_id,
    {{ contract_clean_signals() }},
    fault_code,
    {{ contract_dq_flags() }} as dq_flags,
    ingested_at
from deduped
