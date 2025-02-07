CREATE TABLE
    IF NOT EXISTS data_records (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        type VARCHAR(255) NOT NULL,
        content TEXT NOT NULL,
        metadata TEXT,
        created_at TIMESTAMP NOT NULL,
        updated_at TIMESTAMP NOT NULL
    );