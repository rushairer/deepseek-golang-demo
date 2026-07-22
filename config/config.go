package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultDeepSeekBaseURL = "https://api.deepseek.com"
	defaultDeepSeekModel   = "deepseek-v4-flash"
)

type Config struct {
	Port                  string
	DBDSN                 string
	APIAuthToken          string
	RateLimitPerMinute    int
	MaxRequestBodyBytes   int64
	DeepSeekAPIKey        string
	DeepSeekBaseURL       string
	DeepSeekModel         string
	DeepSeekTimeout       time.Duration
	AgentMaxSteps         int
	AgentMaxOutputTokens  int
	RequireNotifyApproval bool
	SMTPHost              string
	SMTPPort              string
	SMTPUser              string
	SMTPPass              string
	SMTPFrom              string
	SMTPStartTLS          bool
	EmailTargets          map[string]string
	WebhookTargets        map[string]string
	AllowHTTPWebhooks     bool
	AllowPrivateWebhooks  bool
	NotificationTimeout   time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:                  envOr("PORT", "8080"),
		DBDSN:                 strings.TrimSpace(os.Getenv("DB_DSN")),
		APIAuthToken:          strings.TrimSpace(os.Getenv("API_AUTH_TOKEN")),
		RateLimitPerMinute:    envInt("RATE_LIMIT_PER_MINUTE", 60),
		MaxRequestBodyBytes:   envInt64("MAX_REQUEST_BODY_BYTES", 1<<20),
		DeepSeekAPIKey:        strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
		DeepSeekBaseURL:       strings.TrimRight(envOr("DEEPSEEK_BASE_URL", defaultDeepSeekBaseURL), "/"),
		DeepSeekModel:         envOr("DEEPSEEK_MODEL", defaultDeepSeekModel),
		DeepSeekTimeout:       envDuration("DEEPSEEK_TIMEOUT", 90*time.Second),
		AgentMaxSteps:         envInt("AGENT_MAX_STEPS", 6),
		AgentMaxOutputTokens:  envInt("AGENT_MAX_OUTPUT_TOKENS", 4096),
		RequireNotifyApproval: envBool("REQUIRE_NOTIFICATION_APPROVAL", true),
		SMTPHost:              strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:              envOr("SMTP_PORT", "587"),
		SMTPUser:              strings.TrimSpace(os.Getenv("SMTP_USER")),
		SMTPPass:              os.Getenv("SMTP_PASS"),
		SMTPFrom:              strings.TrimSpace(os.Getenv("SMTP_FROM")),
		SMTPStartTLS:          envBool("SMTP_STARTTLS", true),
		AllowHTTPWebhooks:     envBool("ALLOW_HTTP_WEBHOOKS", false),
		AllowPrivateWebhooks:  envBool("ALLOW_PRIVATE_WEBHOOKS", false),
		NotificationTimeout:   envDuration("NOTIFICATION_TIMEOUT", 15*time.Second),
	}

	if cfg.DeepSeekAPIKey == "" {
		return Config{}, fmt.Errorf("DEEPSEEK_API_KEY is required")
	}
	if cfg.DBDSN == "" {
		return Config{}, fmt.Errorf("DB_DSN is required")
	}
	if cfg.AgentMaxSteps < 1 || cfg.AgentMaxSteps > 20 {
		return Config{}, fmt.Errorf("AGENT_MAX_STEPS must be between 1 and 20")
	}
	if cfg.AgentMaxOutputTokens < 256 {
		return Config{}, fmt.Errorf("AGENT_MAX_OUTPUT_TOKENS must be at least 256")
	}

	var err error
	cfg.EmailTargets, err = envStringMap("EMAIL_TARGETS_JSON")
	if err != nil {
		return Config{}, err
	}
	cfg.WebhookTargets, err = envStringMap("WEBHOOK_TARGETS_JSON")
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envStringMap(key string) (map[string]string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return map[string]string{}, nil
	}
	result := map[string]string{}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of string values: %w", key, err)
	}
	return result, nil
}
