CREATE TABLE IF NOT EXISTS approval_requests (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    run_id BIGINT NOT NULL,
    record_id BIGINT NOT NULL,
    tool_call_id VARCHAR(191) NOT NULL,
    tool_name VARCHAR(128) NOT NULL,
    arguments JSON NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    decision_reason TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    decided_at TIMESTAMP NULL,
    FOREIGN KEY (run_id) REFERENCES agent_runs (id) ON DELETE CASCADE,
    FOREIGN KEY (record_id) REFERENCES data_records (id) ON DELETE CASCADE,
    UNIQUE KEY unique_approval_tool_call (run_id, tool_call_id),
    INDEX idx_status_created_at (status, created_at)
);
