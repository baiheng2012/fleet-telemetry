CREATE TABLE IF NOT EXISTS semantic.ml_features (
    vehicle_id        TEXT        NOT NULL,
    bucket            TIMESTAMPTZ NOT NULL,      -- 5 分钟桶起点
    vehicle_type      TEXT,
    running           BOOLEAN,
    rpm_mean          DOUBLE PRECISION,
    coolant_mean      DOUBLE PRECISION,
    hyd_mean          DOUBLE PRECISION,
    fuel_min          DOUBLE PRECISION,
    fault_count       INTEGER,
    coolant_mean_30m  DOUBLE PRECISION,
    coolant_std_30m   DOUBLE PRECISION,
    hyd_mean_30m      DOUBLE PRECISION,
    hyd_std_30m       DOUBLE PRECISION,
    coolant_delta_30m DOUBLE PRECISION,
    PRIMARY KEY (vehicle_id, bucket)
);

CREATE TABLE IF NOT EXISTS semantic.anomalies (
    vehicle_id TEXT        NOT NULL,
    signal     TEXT        NOT NULL,
    bucket     TIMESTAMPTZ NOT NULL,
    value      DOUBLE PRECISION,
    baseline   DOUBLE PRECISION,
    z          DOUBLE PRECISION,
    PRIMARY KEY (vehicle_id, signal, bucket)
);

