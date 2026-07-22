package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deepseek_golang_demo/agent"
	"deepseek_golang_demo/api"
	"deepseek_golang_demo/config"
	"deepseek_golang_demo/models"
	"deepseek_golang_demo/services/actions"
	"deepseek_golang_demo/services/deepseek"
	"deepseek_golang_demo/services/notification"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func runDatabaseMigrations(db *sql.DB) (*migrate.Migrate, error) {
	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		return nil, fmt.Errorf("create migration driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://migrations", "mysql", driver)
	if err != nil {
		return nil, fmt.Errorf("create migration instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return m, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	db, err := models.NewDB(cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer db.Close()
	m, err := runDatabaseMigrations(db)
	if err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	defer m.Close()

	store := models.NewStore(db)
	client := deepseek.NewClient(cfg.DeepSeekAPIKey, cfg.DeepSeekBaseURL, cfg.DeepSeekModel, cfg.DeepSeekTimeout)
	notifier := notification.NewSender(notification.Config{SMTPHost: cfg.SMTPHost, SMTPPort: cfg.SMTPPort, SMTPUser: cfg.SMTPUser, SMTPPass: cfg.SMTPPass, SMTPFrom: cfg.SMTPFrom, SMTPStartTLS: cfg.SMTPStartTLS, EmailTargets: cfg.EmailTargets, WebhookTargets: cfg.WebhookTargets, AllowHTTPWebhooks: cfg.AllowHTTPWebhooks, AllowPrivateWebhooks: cfg.AllowPrivateWebhooks, Timeout: cfg.NotificationTimeout})
	executor := actions.NewExecutor(store, notifier, cfg.RequireNotifyApproval)
	runner := agent.NewRunner(client, executor, store, cfg.AgentMaxSteps, cfg.AgentMaxOutputTokens)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	server := api.NewServer(store, runner, executor, cfg.MaxRequestBodyBytes)
	server.SetupRoutes(router, cfg.APIAuthToken, cfg.RateLimitPerMinute)

	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: cfg.DeepSeekTimeout + 15*time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Printf("server listening on %s with model %s", httpServer.Addr, cfg.DeepSeekModel)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve HTTP: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
