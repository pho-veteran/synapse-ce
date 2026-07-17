-- +goose Up
WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY engagement_id ORDER BY started_at DESC, id DESC) AS n
    FROM scan_jobs
    WHERE status = 'running'
)
UPDATE scan_jobs
SET status = 'failed', stage = 'superseded', progress = 100,
    error = 'superseded by a newer running scan during one-running-job migration',
    finished_at = now()
WHERE id IN (SELECT id FROM ranked WHERE n > 1);

CREATE UNIQUE INDEX scan_jobs_one_running_per_engagement
    ON scan_jobs (engagement_id) WHERE status = 'running';

-- +goose Down
DROP INDEX scan_jobs_one_running_per_engagement;
