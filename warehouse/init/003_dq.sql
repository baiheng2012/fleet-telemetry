CREATE SCHEMA IF NOT EXISTS dq;

CREATE TABLE IF NOT EXISTS dq.check_results (
    id             BIGSERIAL        PRIMARY KEY,
    run_id         UUID             NOT NULL,
    checked_at     TIMESTAMPTZ      NOT NULL DEFAULT now(),
    check_name     TEXT             NOT NULL,
    scope          TEXT             NOT NULL,   -- 车辆ID / 信号名 / 来源 / 表.字段
    value          DOUBLE PRECISION,
    warn_threshold DOUBLE PRECISION,
    fail_threshold DOUBLE PRECISION,
    status         TEXT             NOT NULL CHECK (status IN ('pass', 'warn', 'fail')),
    details        JSONB
);

CREATE INDEX IF NOT EXISTS idx_dq_results_lookup
    ON dq.check_results (check_name, scope, checked_at DESC);

-- 每个 (检查, 范围) 的最新一次结果
CREATE OR REPLACE VIEW dq.latest_results AS
SELECT DISTINCT ON (check_name, scope) *
FROM dq.check_results
ORDER BY check_name, scope, checked_at DESC;

-- 每次运行的汇总
CREATE OR REPLACE VIEW dq.run_summary AS
SELECT run_id,
       min(checked_at)                         AS checked_at,
       count(*) FILTER (WHERE status = 'pass') AS passed,
       count(*) FILTER (WHERE status = 'warn') AS warned,
       count(*) FILTER (WHERE status = 'fail') AS failed
FROM dq.check_results
GROUP BY run_id;
