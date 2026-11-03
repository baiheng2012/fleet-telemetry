CREATE TABLE IF NOT EXISTS raw.telemetry_history (
    batch_id  TEXT        NOT NULL,
    row_num   BIGINT      NOT NULL,
    loaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload   JSONB       NOT NULL,
    PRIMARY KEY (batch_id, row_num)
);
