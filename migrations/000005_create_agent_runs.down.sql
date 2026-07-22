DROP TABLE IF EXISTS agent_steps;
DROP TABLE IF EXISTS agent_runs;
ALTER TABLE notifications
    DROP INDEX idx_record_created_at,
    DROP INDEX unique_notification_idempotency,
    DROP COLUMN idempotency_key,
    DROP COLUMN target;
