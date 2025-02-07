package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"deepseek_golang_demo/models"
	"deepseek_golang_demo/services/actions"
	"deepseek_golang_demo/services/deepseek"

	"github.com/gin-gonic/gin"
)

type Server struct {
	db          *sql.DB
	deepseekCli *deepseek.Client
}

func NewServer(db *sql.DB, deepseekCli *deepseek.Client) *Server {
	return &Server{
		db:          db,
		deepseekCli: deepseekCli,
	}
}

func (s *Server) SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")
	api.POST("/analyze/:id", s.HandleAnalyzeData)
	api.POST("/records", s.HandleCreateRecord)
	api.GET("/records/:id", s.HandleGetRecord)
}

func (s *Server) HandleAnalyzeData(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		log.Printf("无效的记录ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的记录ID"})
		return
	}

	record, err := models.GetDataRecord(s.db, id)
	if err != nil {
		log.Printf("获取记录失败 (ID: %d): %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取记录失败: %v", err)})
		return
	}
	if record == nil {
		log.Printf("记录未找到 (ID: %d)", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "记录未找到"})
		return
	}

	// 构建分析提示词
	prompt := fmt.Sprintf("请分析以下%s类型的数据：\n%s", record.Type, record.Content)

	// 调用DeepSeek API进行分析
	response, err := s.deepseekCli.AnalyzeData(prompt, record)
	if err != nil {
		log.Printf("数据分析失败 (ID: %d): %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("数据分析失败: %v", err)})
		return
	}

	// 保存分析结果
	result := &models.AnalysisResult{
		RecordID:    id,
		Analysis:    response.Analysis,
		Suggestions: response.Suggestions,
		Confidence:  response.Confidence,
	}

	if err := models.SaveAnalysisResult(s.db, result); err != nil {
		log.Printf("保存分析结果失败 (ID: %d): %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存分析结果失败: %v", err)})
		return
	}

	// 执行建议的操作
	for i, action := range response.Actions {
		if err := actions.ExecuteAction(action, s.db); err != nil {
			log.Printf("执行操作失败 (ID: %d, 操作索引: %d, 类型: %s): %v", id, i, action.Type, err)
		}
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) HandleCreateRecord(c *gin.Context) {
	var record models.DataRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := models.CreateDataRecord(s.db, &record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error creating record: %v", err)})
		return
	}

	c.JSON(http.StatusOK, record)
}

func (s *Server) HandleGetRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid record ID"})
		return
	}

	record, err := models.GetDataRecord(s.db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error getting record: %v", err)})
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		return
	}

	c.JSON(http.StatusOK, record)
}
