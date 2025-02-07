package models

import (
	"time"
)

// DataRecord 表示需要分析的数据记录
type DataRecord struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`      // 数据类型
	Content   string    `json:"content"`   // 数据内容
	Metadata  string    `json:"metadata"`  // 元数据
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// AnalysisResult 表示数据分析结果
type AnalysisResult struct {
	RecordID    int64     `json:"recordId"`    // 关联的数据记录ID
	Analysis    string    `json:"analysis"`    // 分析结果
	Suggestions []string  `json:"suggestions"` // 建议操作
	Confidence  float64   `json:"confidence"`  // 置信度
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
}
