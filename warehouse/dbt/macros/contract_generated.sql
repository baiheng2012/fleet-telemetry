{# GENERATED from contracts/vehicle_semantics.yaml by fleet_tools.contract_codegen. DO NOT EDIT. #}

{% macro contract_extract_signals() %}
    (payload->>'engine_rpm')::double precision as engine_rpm,
    (payload->>'coolant_temp')::double precision as coolant_temp,
    (payload->>'hydraulic_pressure')::double precision as hydraulic_pressure,
    (payload->>'fuel_level')::double precision as fuel_level,
    (payload->>'hour_meter')::double precision as hour_meter
{% endmacro %}

{% macro contract_clean_signals() %}
    case when engine_rpm between 0 and 3000 then engine_rpm end as engine_rpm,
    case when coolant_temp between -20 and 130 then coolant_temp end as coolant_temp,
    case when hydraulic_pressure between 0 and 350 then hydraulic_pressure end as hydraulic_pressure,
    case when fuel_level between 0 and 100 then fuel_level end as fuel_level,
    case when hour_meter between 0 and 100000 then hour_meter end as hour_meter
{% endmacro %}

{% macro contract_dq_flags() %}
    array_remove(array[
        case when engine_rpm is not null and (engine_rpm < 0 or engine_rpm > 3000) then 'engine_rpm_out_of_range' end::text,
        case when coolant_temp is not null and (coolant_temp < -20 or coolant_temp > 130) then 'coolant_temp_out_of_range' end::text,
        case when hydraulic_pressure is not null and (hydraulic_pressure < 0 or hydraulic_pressure > 350) then 'hydraulic_pressure_out_of_range' end::text,
        case when fuel_level is not null and (fuel_level < 0 or fuel_level > 100) then 'fuel_level_out_of_range' end::text,
        case when hour_meter is not null and (hour_meter < 0 or hour_meter > 100000) then 'hour_meter_out_of_range' end::text,
        case when fault_code is not null and fault_code not in ('E101', 'E205', 'E310', 'E412') then 'unknown_fault_code' end::text
    ], null)
{% endmacro %}
