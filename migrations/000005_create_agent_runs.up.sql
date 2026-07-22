ALTER TABLE notifications
    ADD COLUMN target VARCHAR(64) NOT NULL DEFAULT '' AFTER channel,
    ADD COLUMN idempotency_key VARCHAR(191) NULL AFTER status,
    ADD UNIQUE KEY unique_notification_idempotency (idempotency_key),
    ADD INDEX idx_record_created_at (record_id, created_at);

CREATE TABLE IF NOT EXISTS agent_runs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    record_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL,
    model VARCHAR(128) NOT NULL,
    prompt_version VARCHAR(64) NOT NULL,
    max_steps INT NOT NULL,
    steps_used INT NOT NULL DEFAULT 0,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    error_message TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP NULL,
    FOREIGN KEY (record_id) REFERENCES data_records (id) ON DELETE CASCADE,
    INDEX idx_record_created_at (record_id, created_at),
    INDEX idx_status_updated_at (status, updated_at)
);

CREATE TABLE IF NOT EXISTS agent_steps (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    step_number INT NOT NULL,
    kind VARCHAR(20) NOT NULL,
    tool_call_id VARCHAR(191) NULL,
    tool_name VARCHAR(128) NULL,
    arguments JSON NULL,
    result JSON NULL,
    status VARCHAR(20) NOT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (run_id) REFERENCES agent_runs (id) ON DELETE CASCADE,
    INDEX idx_run_step (run_id, step_number),
    UNIQUE KEY unique_run_tool_call (run_id, tool_call_id)
);
