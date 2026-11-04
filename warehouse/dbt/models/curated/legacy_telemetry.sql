with dedup as (
    select distinct on (lsn, seq) *
    from {{ ref('stg_legacy_cdc') }}
    order by lsn, seq, stream_seq
),

ranked as (
    select *,
           row_number() over (
               partition by (row_data->>'id')::bigint
               order by lsn desc, seq desc
           ) as rn
    from dedup
)

select
    (r.row_data->>'id')::bigint                      as legacy_id,
    m.vehicle_id,
    r.row_data->>'equip_code'                        as equip_code,
    (r.row_data->>'recorded_at')::timestamptz        as recorded_at,
    (r.row_data->>'rpm')::int                        as rpm,
    (r.row_data->>'coolant_c')::numeric              as coolant_c,
    (r.row_data->>'hyd_psi')::numeric                as hyd_psi,
    round((r.row_data->>'hyd_psi')::numeric / 14.5038, 3) as hydraulic_pressure_bar,
    (r.row_data->>'fuel_pct')::numeric               as fuel_pct,
    (r.row_data->>'hours')::numeric                  as hours,
    r.row_data->>'fault'                             as fault
from ranked r
left join {{ ref('legacy_equipment_map') }} m
       on m.equip_code = r.row_data->>'equip_code'
where r.rn = 1 and r.op <> 'd'
