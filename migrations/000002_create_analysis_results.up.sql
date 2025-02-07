CREATE TABLE
    IF NOT EXISTS analysis_results (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        record_id BIGINT NOT NULL,
        analysis TEXT NOT NULL,
        suggestions TEXT NOT NULL,
        confidence DOUBLE NOT NULL,
        created_at TIMESTAMP NOT NULL,
        FOREIGN KEY (record_id) REFERENCES data_records (id)
    );