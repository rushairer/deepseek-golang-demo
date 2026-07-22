package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"deepseek_golang_demo/agent"
	"deepseek_golang_demo/models"
	"deepseek_golang_demo/services/actions"

	"github.com/gin-gonic/gin"
)

type Server struct {
	store               *models.Store
	runner              *agent.Runner
	executor            *actions.Executor
	maxRequestBodyBytes int64
}

func NewServer(store *models.Store, runner *agent.Runner, executor *actions.Executor, maxRequestBodyBytes int64) *Server {
	return &Server{store: store, runner: runner, executor: executor, maxRequestBodyBytes: maxRequestBodyBytes}
}

func (s *Server) SetupRoutes(router *gin.Engine, authToken string, rateLimit int) {
	router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	api := router.Group("/api", authMiddleware(authToken), rateLimitMiddleware(rateLimit))
	api.POST("/records", s.handleCreateRecord)
	api.GET("/records/:id", s.handleGetRecord)
	api.POST("/analyze/:id", s.handleAnalyzeData)
	api.GET("/runs/:id", s.handleGetRun)
	api.POST("/approvals/:id/approve", s.handleApprove)
	api.POST("/approvals/:id/reject", s.handleReject)
}

func (s *Server) handleCreateRecord(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.maxRequestBodyBytes)
	var record models.DataRecord
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	record.Type = strings.TrimSpace(strings.ToLower(record.Type))
	if record.Type != "text" && record.Type != "metric" && record.Type != "log" {
		writeError(c, http.StatusBadRequest, "type must be text, metric, or log", nil)
		return
	}
	record.Content = strings.TrimSpace(record.Content)
	if record.Content == "" {
		writeError(c, http.StatusBadRequest, "content is required", nil)
		return
	}
	if len(record.Content) > 500_000 {
		writeError(c, http.StatusRequestEntityTooLarge, "content is too large", nil)
		return
	}
	if len(record.Metadata) > 0 && !json.Valid(record.Metadata) {
		writeError(c, http.StatusBadRequest, "metadata must be valid JSON", nil)
		return
	}
	if err := s.store.CreateDataRecord(c.Request.Context(), &record); err != nil {
		writeError(c, http.StatusInternalServerError, "create record failed", err)
		return
	}
	c.JSON(http.StatusCreated, record)
}

func (s *Server) handleGetRecord(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	detail, err := s.store.GetRecordDetail(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "get record failed", err)
		return
	}
	if detail == nil {
		writeError(c, http.StatusNotFound, "record not found", nil)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (s *Server) handleAnalyzeData(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	record, err := s.store.GetDataRecord(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "get record failed", err)
		return
	}
	if record == nil {
		writeError(c, http.StatusNotFound, "record not found", nil)
		return
	}
	result, err := s.runner.Run(c.Request.Context(), record)
	if err != nil {
		writeError(c, http.StatusBadGateway, "agent run failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleGetRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	run, steps, err := s.store.GetAgentRun(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "get agent run failed", err)
		return
	}
	if run == nil {
		writeError(c, http.StatusNotFound, "agent run not found", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"run": run, "steps": steps})
}

type decisionRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleApprove(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var request decisionRequest
	_ = c.ShouldBindJSON(&request)
	approval, err := s.store.GetApproval(c.Request.Context(), id)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "get approval failed", err)
		return
	}
	if approval == nil {
		writeError(c, http.StatusNotFound, "approval not found", nil)
		return
	}
	if approval.Status != "pending" {
		writeError(c, http.StatusConflict, "approval is already decided", nil)
		return
	}
	result := s.executor.ExecuteApprovedNotification(c.Request.Context(), approval)
	if result.Status != "succeeded" {
		writeError(c, http.StatusBadGateway, "approved action failed", errors.New(result.Error))
		return
	}
	if err := s.store.DecideApproval(c.Request.Context(), id, "approved", strings.TrimSpace(request.Reason)); err != nil {
		writeError(c, http.StatusConflict, "approval decision failed", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (s *Server) handleReject(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var request decisionRequest
	_ = c.ShouldBindJSON(&request)
	if err := s.store.DecideApproval(c.Request.Context(), id, "rejected", strings.TrimSpace(request.Reason)); err != nil {
		writeError(c, http.StatusConflict, "approval decision failed", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid ID", err)
		return 0, false
	}
	return id, true
}

func writeError(c *gin.Context, status int, message string, err error) {
	requestID := c.GetHeader("X-Request-ID")
	payload := gin.H{"error": message}
	if requestID != "" {
		payload["requestId"] = requestID
	}
	if err != nil {
		fmt.Printf("request failed: %s: %v\n", message, err)
	}
	c.JSON(status, payload)
}
