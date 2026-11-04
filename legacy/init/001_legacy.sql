CREATE SCHEMA IF NOT EXISTS legacy;

CREATE TABLE legacy.telemetry_log (
    id          BIGSERIAL PRIMARY KEY,
    equip_code  TEXT         NOT NULL,        -- L1/L2/T1/T2/B1，不是新平台的 vehicle_id
    recorded_at TIMESTAMPTZ  NOT NULL,
    rpm         INTEGER,
    coolant_c   NUMERIC(6,2),
    hyd_psi     NUMERIC(8,2),                 -- 旧系统用 psi，新平台用 bar
    fuel_pct    NUMERIC(5,2),
    hours       NUMERIC(10,3),
    fault       TEXT
);

-- 更新/删除事件携带完整旧行，对账和排查都需要
ALTER TABLE legacy.telemetry_log REPLICA IDENTITY FULL;

CREATE PUBLICATION fleet_cdc FOR TABLE legacy.telemetry_log;
