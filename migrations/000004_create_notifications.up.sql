CREATE TABLE
    IF NOT EXISTS notifications (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        record_id BIGINT NOT NULL,
        channel VARCHAR(50) NOT NULL,
        message TEXT NOT NULL,
        status VARCHAR(20) NOT NULL DEFAULT 'pending',
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        sent_at TIMESTAMP NULL,
        FOREIGN KEY (record_id) REFERENCES data_records (id) ON DELETE CASCADE,
        INDEX idx_status (status),
        INDEX idx_created_at (created_at)
    );