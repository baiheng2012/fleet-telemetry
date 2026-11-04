{% macro create_active_alerts_view() %}
create or replace view dq.active_alerts as
select 'dq'::text as source,
       check_name || ' / ' || scope as alert,
       status as severity,
       checked_at as at,
       value
from dq.latest_results
where status in ('fail', 'warn')
union all
select 'anomaly',
       vehicle_id || ' ' || signal || ' z=' || round(z::numeric, 1),
       'warn', bucket, z
from semantic.anomalies
where bucket > now() - interval '7 days'
union all
select 'fault',
       vehicle_id || ' fault ' || fault_code,
       'fail', ts, null::float8
from (
    select distinct on (vehicle_id) vehicle_id, fault_code, ts
    from {{ ref('curated_telemetry') }}
    where fault_code is not null and ts > now() - interval '24 hours'
    order by vehicle_id, ts desc
) f
{% endmacro %}
