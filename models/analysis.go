package models

import (
	"database/sql"
	"time"
)

// TextAnalysisResult 文本分析结果
type TextAnalysisResult struct {
	Summary   string   `json:"summary"`
	Entities  []string `json:"entities"`
	Sentiment string   `json:"sentiment"`
	Urgency   int      `json:"urgency"`
	Actions   []Action `json:"actions"`
}

// MetricsAnalysisResult 指标分析结果
type MetricsAnalysisResult struct {
	Stats     map[string]Stats `json:"stats"`
	Anomalies []Anomaly        `json:"anomalies"`
	Trend     string           `json:"trend"`
	Actions   []Action         `json:"actions"`
}

// LogAnalysisResult 日志分析结果
type LogAnalysisResult struct {
	Level      string   `json:"level"`
	ErrorCode  string   `json:"error_code"`
	Message    string   `json:"message"`
	StackTrace string   `json:"stack_trace"`
	Frequency  string   `json:"frequency"`
	Impact     string   `json:"impact"`
	Actions    []Action `json:"actions"`
}

// Stats 统计数据
type Stats struct {
	Avg    float64 `json:"avg"`
	Median float64 `json:"median"`
	Std    float64 `json:"std"`
}

// Anomaly 异常数据
type Anomaly struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Severity  int     `json:"severity"`
}

// Action 建议操作
type Action struct {
	Type     string                 `json:"type"`
	Target   string                 `json:"target"`
	Params   map[string]interface{} `json:"params"`
	Priority int                    `json:"priority"`
	Rollback string                 `json:"rollback,omitempty"`
}

// Tag 数据标签
type Tag struct {
	ID        int64     `json:"id"`
	RecordID  int64     `json:"record_id"`
	TagName   string    `json:"tag_name"`
	CreatedAt time.Time `json:"created_at"`
}

// Notification 通知记录
type Notification struct {
	ID        int64     `json:"id"`
	RecordID  int64     `json:"record_id"`
	Channel   string    `json:"channel"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	SentAt    time.Time `json:"sent_at,omitempty"`
}

// UpdateStatus 更新数据记录状态
func UpdateStatus(db *sql.DB, id string, status string) error {
	_, err := db.Exec("UPDATE data_records SET metadata = JSON_SET(COALESCE(metadata, '{}'), '$.status', ?) WHERE id = ?", status, id)
	return err
}

// AddTag 添加标签
func AddTag(db *sql.DB, recordID string, tagName string) error {
	_, err := db.Exec(
		"INSERT INTO tags (record_id, tag_name, created_at) VALUES (?, ?, ?)",
		recordID, tagName, time.Now(),
	)
	return err
}

// CreateNotification 创建通知
func CreateNotification(db *sql.DB, recordID int64, channel string, message string) error {
	_, err := db.Exec(
		"INSERT INTO notifications (record_id, channel, message, status, created_at) VALUES (?, ?, ?, ?, ?)",
		recordID, channel, message, "pending", time.Now(),
	)
	return err
}

// UpdateNotificationStatus 更新通知状态
func UpdateNotificationStatus(db *sql.DB, id int64, status string) error {
	var sentAt interface{}
	if status == "sent" {
		sentAt = time.Now()
	}
	_, err := db.Exec(
		"UPDATE notifications SET status = ?, sent_at = ? WHERE id = ?",
		status, sentAt, id,
	)
	return err
}

// GetTagsByRecordID 获取记录的所有标签
func GetTagsByRecordID(db *sql.DB, recordID int64) ([]Tag, error) {
	rows, err := db.Query("SELECT id, record_id, tag_name, created_at FROM tags WHERE record_id = ?", recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.RecordID, &tag.TagName, &tag.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// GetPendingNotifications 获取待处理的通知
func GetPendingNotifications(db *sql.DB) ([]Notification, error) {
	rows, err := db.Query(
		"SELECT id, record_id, channel, message, status, created_at, sent_at FROM notifications WHERE status = 'pending'",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var notification Notification
		var sentAt sql.NullTime
		if err := rows.Scan(
			&notification.ID,
			&notification.RecordID,
			&notification.Channel,
			&notification.Message,
			&notification.Status,
			&notification.CreatedAt,
			&sentAt,
		); err != nil {
			return nil, err
		}
		if sentAt.Valid {
			notification.SentAt = sentAt.Time
		}
		notifications = append(notifications, notification)
	}
	return notifications, nil
}
