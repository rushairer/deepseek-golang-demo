package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"deepseek_golang_demo/api"
	"deepseek_golang_demo/models"
	"deepseek_golang_demo/prompts"
	"deepseek_golang_demo/services/deepseek"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

// runDatabaseMigrations 执行数据库迁移
func runDatabaseMigrations(db *sql.DB) (*migrate.Migrate, error) {
	// 创建file source实例
	fsrc, err := (&file.File{}).Open("file://migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create file source: %v", err)
	}

	// 创建mysql driver实例
	config := mysql.Config{}
	driver, err := mysql.WithInstance(db, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql driver: %v", err)
	}

	// 创建migrate实例
	m, err := migrate.NewWithInstance(
		"file", fsrc,
		"mysql", driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %v", err)
	}

	// 执行迁移
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, fmt.Errorf("failed to run migrations: %v", err)
	}

	return m, nil
}

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// 获取配置
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	dbDSN := os.Getenv("DB_DSN")
	if apiKey == "" || dbDSN == "" {
		log.Fatal("DEEPSEEK_API_KEY and DB_DSN must be set in .env file")
	}

	// 初始化数据库连接
	db, err := models.NewDB(dbDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 运行数据库迁移
	m, err := runDatabaseMigrations(db)
	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	// 在应用退出时关闭迁移实例
	defer func() {
		if m != nil {
			sourceErr, dbErr := m.Close()
			if sourceErr != nil {
				log.Printf("Error closing migration source: %v", sourceErr)
			}
			if dbErr != nil {
				log.Printf("Error closing migration database: %v", dbErr)
			}
		}
	}()

	// 初始化DeepSeek客户端
	deepseekCli := deepseek.NewClient(apiKey)

	// 初始化提示词模板管理器
	templateManager := prompts.NewTemplateManager()
	for _, template := range prompts.DefaultTemplates() {
		templateManager.RegisterTemplate(template)
	}

	// 初始化HTTP服务器
	server := api.NewServer(db, deepseekCli)
	router := gin.Default()
	server.SetupRoutes(router)

	// 启动服务器
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s...\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
