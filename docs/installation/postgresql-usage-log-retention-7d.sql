-- Keep new-api database usage logs for 7 days.
--
-- Run this script while connected to the database used by LOG_SQL_DSN. If
-- LOG_SQL_DSN is unset, connect to the database used by SQL_DSN instead.
-- The pg_cron extension must be installed and preloaded by the PostgreSQL
-- server before this script is executed. The server's cron.database_name must
-- be the same database as the current connection.

CREATE EXTENSION IF NOT EXISTS pg_cron;

-- Make the script idempotent on pg_cron versions that do not update named
-- jobs in place.
DO $unschedule$
DECLARE
    existing_job_id bigint;
BEGIN
    FOR existing_job_id IN
        SELECT jobid
        FROM cron.job
        WHERE jobname = 'new-api-usage-log-retention-7d'
          AND username = CURRENT_USER
    LOOP
        PERFORM cron.unschedule(existing_job_id);
    END LOOP;
END
$unschedule$;

-- Run at minute 17 of every hour. Each run retains the latest 7 * 24 hours,
-- so the maximum cleanup delay is less than one hour and is timezone-neutral.
SELECT cron.schedule(
    'new-api-usage-log-retention-7d',
    '17 * * * *',
    $command$
        WITH cutoff AS (
            SELECT (FLOOR(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP))::bigint - 7 * 24 * 60 * 60) AS cutoff_epoch
        ),
        deleted_request_details AS (
            DELETE FROM public.request_details AS request_detail
            USING cutoff
            WHERE request_detail.created_at < cutoff.cutoff_epoch
            RETURNING request_detail.id
        )
        DELETE FROM public.logs AS usage_log
        USING cutoff
        WHERE usage_log.created_at < cutoff.cutoff_epoch;
    $command$
);

-- Verify that the job exists and is active.
SELECT jobid, jobname, schedule, database, username, active
FROM cron.job
WHERE jobname = 'new-api-usage-log-retention-7d';

-- Check executions after the first scheduled run.
-- SELECT jobid, status, return_message, start_time, end_time
-- FROM cron.job_run_details
-- WHERE jobid = (
--     SELECT jobid
--     FROM cron.job
--     WHERE jobname = 'new-api-usage-log-retention-7d'
--       AND username = CURRENT_USER
-- )
-- ORDER BY start_time DESC
-- LIMIT 10;
