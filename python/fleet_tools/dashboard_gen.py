"""Generate the Grafana dashboard JSON (dashboards as code)."""
import json

from .contract import REPO_ROOT

DS = {"type": "grafana-postgresql-datasource", "uid": "warehouse"}
OUT = REPO_ROOT / "grafana" / "dashboards" / "fleet.json"


def panel(pid, title, kind, x, y, w, h, sql, fmt="table"):
    return {
        "id": pid, "title": title, "type": kind, "datasource": DS,
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "targets": [{"refId": "A", "datasource": DS, "format": fmt, "rawQuery": True,
                     "editorMode": "code", "rawSql": sql}],
        "fieldConfig": {"defaults": {}, "overrides": []}, "options": {},
    }


def build() -> dict:
    p = [
        panel(1, "Alerts: fail", "stat", 0, 0, 4, 4,
              "select count(*) as fail from dq.active_alerts where severity = 'fail'"),
        panel(2, "Alerts: warn", "stat", 4, 0, 4, 4,
              "select count(*) as warn from dq.active_alerts where severity = 'warn'"),
        panel(3, "Vehicles reporting (<10 min)", "stat", 8, 0, 4, 4,
              "select count(*) as vehicles from dq.latest_results "
              "where check_name = 'freshness_seconds' and value < 600"),
        panel(4, "Active alerts", "table", 12, 0, 12, 8,
              "select source, severity, alert, at from dq.active_alerts "
              "order by (severity = 'fail') desc, at desc"),
        panel(5, "Daily utilization (engine hours / 24h)", "timeseries", 0, 4, 12, 8,
              "select day::timestamp as time, vehicle_id as metric, utilization_ratio as value "
              "from semantic.fct_vehicle_daily where $__timeFilter(day::timestamp) order by 1", "time_series"),
        panel(6, "Engine hours per hour (hour meter)", "timeseries", 0, 12, 12, 8,
              "select hour_start_utc as time, vehicle_id as metric, engine_hours as value "
              "from semantic.fct_vehicle_hourly where $__timeFilter(hour_start_utc) order by 1", "time_series"),
        panel(7, "Coolant temperature, running only (5 min mean)", "timeseries", 12, 8, 12, 8,
              "select bucket as time, vehicle_id as metric, coolant_mean as value "
              "from semantic.ml_features where running and $__timeFilter(bucket) order by 1", "time_series"),
        panel(8, "OEM KPIs per vehicle", "table", 0, 20, 12, 7,
              "select vehicle_id, round(avg(utilization_ratio)::numeric, 3) as avg_utilization, "
              "round(sum(engine_hours)::numeric, 1) as engine_hours, "
              "round((100.0 * sum(fault_samples) / nullif(sum(samples), 0))::numeric, 3) as fault_pct "
              "from semantic.fct_vehicle_daily where $__timeFilter(day::timestamp) group by 1 order by 1"),
        panel(9, "Fault samples per vehicle", "barchart", 12, 16, 12, 8,
              "select vehicle_id, sum(fault_samples)::int as fault_samples "
              "from semantic.fct_vehicle_daily where $__timeFilter(day::timestamp) group by 1 order by 1"),
        panel(10, "Freshness SLA (seconds since last message)", "table", 0, 27, 12, 7,
              "select scope as vehicle, round(value::numeric, 1) as seconds, status "
              "from dq.latest_results where check_name = 'freshness_seconds' order by scope"),
        panel(11, "Data-quality rates (latest)", "table", 12, 24, 12, 10,
              "select check_name, scope, round(value::numeric, 5) as value, status "
              "from dq.latest_results where check_name not in ('freshness_seconds') "
              "order by (status = 'pass'), check_name, scope"),
        panel(12, "Anomalies (robust z-score)", "table", 0, 34, 24, 7,
              "select bucket, vehicle_id, signal, round(value::numeric, 1) as value, "
              "round(baseline::numeric, 1) as baseline, round(z::numeric, 1) as z "
              "from semantic.anomalies order by bucket desc limit 50"),
    ]
    return {
        "uid": "fleet-overview", "title": "Fleet Telemetry Overview", "schemaVersion": 39,
        "time": {"from": "now-14d", "to": "now"}, "refresh": "1m", "editable": True,
        "templating": {"list": []}, "panels": p,
    }


def main() -> int:
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(build(), indent=2) + "\n", encoding="utf-8")
    print("wrote", OUT.relative_to(REPO_ROOT))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
