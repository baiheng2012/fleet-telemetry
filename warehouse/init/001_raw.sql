CREATE SCHEMA IF NOT EXISTS raw;
CREATE SCHEMA IF NOT EXISTS curated;
CREATE SCHEMA IF NOT EXISTS semantic;

CREATE TABLE IF NOT EXISTS raw.telemetry_events (
    stream_seq   BIGINT      PRIMARY KEY,          -- JetStream 流序号，幂等键
    subject      TEXT        NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,             -- 消息进入 NATS 的时间
    ingested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload      JSONB       NOT NULL              -- 原始消息，不做任何加工
);

CREATE INDEX IF NOT EXISTS idx_raw_telemetry_published_at
    ON raw.telemetry_events (published_at);
