package models

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func NewDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	return db, nil
}

func CreateDataRecord(db *sql.DB, record *DataRecord) error {
	query := `INSERT INTO data_records (type, content, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`

	now := time.Now()
	record.CreatedAt = now
	record.UpdatedAt = now

	result, err := db.Exec(query, record.Type, record.Content, record.Metadata,
		record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("error creating data record: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("error getting last insert id: %v", err)
	}

	record.ID = id
	return nil
}

func GetDataRecord(db *sql.DB, id int64) (*DataRecord, error) {
	query := `SELECT id, type, content, metadata, created_at, updated_at
		FROM data_records WHERE id = ?`

	record := &DataRecord{}
	err := db.QueryRow(query, id).Scan(
		&record.ID, &record.Type, &record.Content, &record.Metadata,
		&record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting data record: %v", err)
	}

	return record, nil
}

func SaveAnalysisResult(db *sql.DB, result *AnalysisResult) error {
	query := `INSERT INTO analysis_results (record_id, analysis, suggestions, confidence, created_at)
		VALUES (?, ?, ?, ?, ?)`

	result.CreatedAt = time.Now()

	_, err := db.Exec(query, result.RecordID, result.Analysis,
		fmt.Sprintf("%v", result.Suggestions), result.Confidence, result.CreatedAt)
	if err != nil {
		return fmt.Errorf("error saving analysis result: %v", err)
	}

	return nil
}
