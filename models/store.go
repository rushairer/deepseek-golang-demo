package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct{ db *sql.DB }

func NewDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

type DataRecord struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Content   string          `json:"content"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}
type AnalysisResult struct {
	ID          int64     `json:"id"`
	RecordID    int64     `json:"recordId"`
	Analysis    string    `json:"analysis"`
	Suggestions []string  `json:"suggestions"`
	Confidence  float64   `json:"confidence"`
	CreatedAt   time.Time `json:"createdAt"`
}
type Tag struct {
	ID        int64     `json:"id"`
	RecordID  int64     `json:"recordId"`
	TagName   string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}
type Notification struct {
	ID             int64      `json:"id"`
	RecordID       int64      `json:"recordId"`
	Channel        string     `json:"channel"`
	Target         string     `json:"target"`
	Message        string     `json:"message"`
	Status         string     `json:"status"`
	IdempotencyKey string     `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	SentAt         *time.Time `json:"sentAt,omitempty"`
}
type AgentRun struct {
	ID            int64      `json:"id"`
	RecordID      int64      `json:"recordId"`
	Status        string     `json:"status"`
	Model         string     `json:"model"`
	PromptVersion string     `json:"promptVersion"`
	MaxSteps      int        `json:"maxSteps"`
	StepsUsed     int        `json:"stepsUsed"`
	InputTokens   int        `json:"inputTokens"`
	OutputTokens  int        `json:"outputTokens"`
	ErrorMessage  string     `json:"errorMessage,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}
type AgentStep struct {
	ID           int64     `json:"id"`
	RunID        int64     `json:"runId"`
	StepNumber   int       `json:"stepNumber"`
	Kind         string    `json:"kind"`
	ToolCallID   string    `json:"toolCallId,omitempty"`
	ToolName     string    `json:"toolName,omitempty"`
	Arguments    string    `json:"arguments,omitempty"`
	Result       string    `json:"result,omitempty"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}
type ApprovalRequest struct {
	ID             int64      `json:"id"`
	RunID          int64      `json:"runId"`
	RecordID       int64      `json:"recordId"`
	ToolCallID     string     `json:"toolCallId"`
	ToolName       string     `json:"toolName"`
	Arguments      string     `json:"arguments"`
	Status         string     `json:"status"`
	DecisionReason string     `json:"decisionReason,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	DecidedAt      *time.Time `json:"decidedAt,omitempty"`
}
type RecordDetail struct {
	DataRecord
	Analysis      *AnalysisResult `json:"analysis,omitempty"`
	Tags          []Tag           `json:"tags"`
	Notifications []Notification  `json:"notifications"`
	Runs          []AgentRun      `json:"runs"`
}

func (s *Store) CreateDataRecord(ctx context.Context, r *DataRecord) error {
	if len(r.Metadata) == 0 {
		r.Metadata = json.RawMessage(`{}`)
	}
	now := time.Now().UTC()
	x, err := s.db.ExecContext(ctx, `INSERT INTO data_records (type,content,metadata,created_at,updated_at) VALUES (?,?,?,?,?)`, r.Type, r.Content, []byte(r.Metadata), now, now)
	if err != nil {
		return fmt.Errorf("create data record: %w", err)
	}
	r.ID, err = x.LastInsertId()
	r.CreatedAt, r.UpdatedAt = now, now
	return err
}
func (s *Store) GetDataRecord(ctx context.Context, id int64) (*DataRecord, error) {
	var r DataRecord
	var m []byte
	err := s.db.QueryRowContext(ctx, `SELECT id,type,content,COALESCE(metadata,'{}'),created_at,updated_at FROM data_records WHERE id=?`, id).Scan(&r.ID, &r.Type, &r.Content, &m, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get record: %w", err)
	}
	r.Metadata = m
	return &r, nil
}
func (s *Store) SaveAnalysisResult(ctx context.Context, r *AnalysisResult) error {
	b, err := json.Marshal(r.Suggestions)
	if err != nil {
		return err
	}
	r.CreatedAt = time.Now().UTC()
	x, err := s.db.ExecContext(ctx, `INSERT INTO analysis_results (record_id,analysis,suggestions,confidence,created_at) VALUES (?,?,?,?,?)`, r.RecordID, r.Analysis, b, r.Confidence, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}
	r.ID, err = x.LastInsertId()
	return err
}
func (s *Store) UpdateStatus(ctx context.Context, id int64, status string) error {
	x, err := s.db.ExecContext(ctx, `UPDATE data_records SET metadata=JSON_SET(IF(JSON_VALID(metadata),metadata,'{}'),'$.status',?),updated_at=? WHERE id=?`, status, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	return affected(x, "record")
}
func (s *Store) AddTag(ctx context.Context, id int64, tag string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tags (record_id,tag_name,created_at) VALUES (?,?,?) ON DUPLICATE KEY UPDATE tag_name=VALUES(tag_name)`, id, tag, time.Now().UTC())
	return err
}
func (s *Store) CreateNotification(ctx context.Context, n Notification) (int64, bool, error) {
	x, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO notifications (record_id,channel,target,message,status,idempotency_key,created_at) VALUES (?,?,?,?,'pending',?,?)`, n.RecordID, n.Channel, n.Target, n.Message, n.IdempotencyKey, time.Now().UTC())
	if err != nil {
		return 0, false, err
	}
	id, err := x.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	if id != 0 {
		return id, true, nil
	}
	err = s.db.QueryRowContext(ctx, `SELECT id FROM notifications WHERE idempotency_key=?`, n.IdempotencyKey).Scan(&id)
	return id, false, err
}
func (s *Store) UpdateNotificationStatus(ctx context.Context, id int64, status string) error {
	var sent any
	if status == "sent" {
		sent = time.Now().UTC()
	}
	x, err := s.db.ExecContext(ctx, `UPDATE notifications SET status=?,sent_at=? WHERE id=?`, status, sent, id)
	if err != nil {
		return err
	}
	return affected(x, "notification")
}
func (s *Store) CreateAgentRun(ctx context.Context, recordID int64, model, version string, maxSteps int) (*AgentRun, error) {
	now := time.Now().UTC()
	x, err := s.db.ExecContext(ctx, `INSERT INTO agent_runs (record_id,status,model,prompt_version,max_steps,steps_used,input_tokens,output_tokens,created_at,updated_at) VALUES (?,'running',?,?,?,0,0,0,?,?)`, recordID, model, version, maxSteps, now, now)
	if err != nil {
		return nil, err
	}
	id, err := x.LastInsertId()
	return &AgentRun{ID: id, RecordID: recordID, Status: "running", Model: model, PromptVersion: version, MaxSteps: maxSteps, CreatedAt: now, UpdatedAt: now}, err
}
func (s *Store) AddAgentStep(ctx context.Context, x AgentStep) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_steps (run_id,step_number,kind,tool_call_id,tool_name,arguments,result,status,error_message,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, x.RunID, x.StepNumber, x.Kind, null(x.ToolCallID), null(x.ToolName), null(x.Arguments), null(x.Result), x.Status, null(x.ErrorMessage), time.Now().UTC())
	return err
}
func (s *Store) FinishAgentRun(ctx context.Context, id int64, status string, steps, in, out int, msg string) error {
	now := time.Now().UTC()
	x, err := s.db.ExecContext(ctx, `UPDATE agent_runs SET status=?,steps_used=?,input_tokens=?,output_tokens=?,error_message=?,updated_at=?,completed_at=? WHERE id=?`, status, steps, in, out, null(msg), now, now, id)
	if err != nil {
		return err
	}
	return affected(x, "agent run")
}
func (s *Store) CreateApproval(ctx context.Context, a ApprovalRequest) (int64, error) {
	x, err := s.db.ExecContext(ctx, `INSERT INTO approval_requests (run_id,record_id,tool_call_id,tool_name,arguments,status,created_at) VALUES (?,?,?,?,?,'pending',?)`, a.RunID, a.RecordID, a.ToolCallID, a.ToolName, a.Arguments, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	return x.LastInsertId()
}
func (s *Store) GetApproval(ctx context.Context, id int64) (*ApprovalRequest, error) {
	var a ApprovalRequest
	var d sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id,run_id,record_id,tool_call_id,tool_name,arguments,status,COALESCE(decision_reason,''),created_at,decided_at FROM approval_requests WHERE id=?`, id).Scan(&a.ID, &a.RunID, &a.RecordID, &a.ToolCallID, &a.ToolName, &a.Arguments, &a.Status, &a.DecisionReason, &a.CreatedAt, &d)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if d.Valid {
		a.DecidedAt = &d.Time
	}
	return &a, err
}
func (s *Store) DecideApproval(ctx context.Context, id int64, status, reason string) error {
	x, err := s.db.ExecContext(ctx, `UPDATE approval_requests SET status=?,decision_reason=?,decided_at=? WHERE id=? AND status='pending'`, status, reason, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	return affected(x, "pending approval")
}
func (s *Store) GetAgentRun(ctx context.Context, id int64) (*AgentRun, []AgentStep, error) {
	r, err := s.getRun(ctx, id)
	if err != nil || r == nil {
		return r, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,run_id,step_number,kind,COALESCE(tool_call_id,''),COALESCE(tool_name,''),COALESCE(arguments,''),COALESCE(result,''),status,COALESCE(error_message,''),created_at FROM agent_steps WHERE run_id=? ORDER BY id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []AgentStep{}
	for rows.Next() {
		var x AgentStep
		if err := rows.Scan(&x.ID, &x.RunID, &x.StepNumber, &x.Kind, &x.ToolCallID, &x.ToolName, &x.Arguments, &x.Result, &x.Status, &x.ErrorMessage, &x.CreatedAt); err != nil {
			return nil, nil, err
		}
		out = append(out, x)
	}
	return r, out, rows.Err()
}
func (s *Store) getRun(ctx context.Context, id int64) (*AgentRun, error) {
	var r AgentRun
	var done sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id,record_id,status,model,prompt_version,max_steps,steps_used,input_tokens,output_tokens,COALESCE(error_message,''),created_at,updated_at,completed_at FROM agent_runs WHERE id=?`, id).Scan(&r.ID, &r.RecordID, &r.Status, &r.Model, &r.PromptVersion, &r.MaxSteps, &r.StepsUsed, &r.InputTokens, &r.OutputTokens, &r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt, &done)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if done.Valid {
		r.CompletedAt = &done.Time
	}
	return &r, err
}
func (s *Store) GetRecordDetail(ctx context.Context, id int64) (*RecordDetail, error) {
	r, err := s.GetDataRecord(ctx, id)
	if err != nil || r == nil {
		return nil, err
	}
	d := &RecordDetail{DataRecord: *r, Tags: []Tag{}, Notifications: []Notification{}, Runs: []AgentRun{}}
	var a AnalysisResult
	var b []byte
	e := s.db.QueryRowContext(ctx, `SELECT id,record_id,analysis,suggestions,confidence,created_at FROM analysis_results WHERE record_id=? ORDER BY id DESC LIMIT 1`, id).Scan(&a.ID, &a.RecordID, &a.Analysis, &b, &a.Confidence, &a.CreatedAt)
	if e == nil {
		_ = json.Unmarshal(b, &a.Suggestions)
		d.Analysis = &a
	} else if e != sql.ErrNoRows {
		return nil, e
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,record_id,tag_name,created_at FROM tags WHERE record_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x Tag
		_ = rows.Scan(&x.ID, &x.RecordID, &x.TagName, &x.CreatedAt)
		d.Tags = append(d.Tags, x)
	}
	rows.Close()
	nrows, err := s.db.QueryContext(ctx, `SELECT id,record_id,channel,target,message,status,COALESCE(idempotency_key,''),created_at,sent_at FROM notifications WHERE record_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	for nrows.Next() {
		var n Notification
		var sent sql.NullTime
		_ = nrows.Scan(&n.ID, &n.RecordID, &n.Channel, &n.Target, &n.Message, &n.Status, &n.IdempotencyKey, &n.CreatedAt, &sent)
		if sent.Valid {
			n.SentAt = &sent.Time
		}
		d.Notifications = append(d.Notifications, n)
	}
	nrows.Close()
	rrows, err := s.db.QueryContext(ctx, `SELECT id FROM agent_runs WHERE record_id=? ORDER BY id DESC LIMIT 20`, id)
	if err != nil {
		return nil, err
	}
	for rrows.Next() {
		var rid int64
		_ = rrows.Scan(&rid)
		run, _ := s.getRun(ctx, rid)
		if run != nil {
			d.Runs = append(d.Runs, *run)
		}
	}
	rrows.Close()
	return d, nil
}
func affected(x sql.Result, name string) error {
	n, err := x.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s not found or not mutable", name)
	}
	return nil
}
func null(v string) any {
	if v == "" {
		return nil
	}
	return v
}
