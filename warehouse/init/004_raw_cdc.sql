CREATE TABLE IF NOT EXISTS raw.legacy_cdc_events (
    stream_seq   BIGINT      PRIMARY KEY,
    subject      TEXT        NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    ingested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload      JSONB       NOT NULL
);

