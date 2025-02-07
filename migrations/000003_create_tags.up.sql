CREATE TABLE
    IF NOT EXISTS tags (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        record_id BIGINT NOT NULL,
        tag_name VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (record_id) REFERENCES data_records (id) ON DELETE CASCADE,
        UNIQUE KEY unique_record_tag (record_id, tag_name),
        INDEX idx_created_at (created_at)
    );