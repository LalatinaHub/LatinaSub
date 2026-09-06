package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configurations for LatinaSub.
type Config struct {
	AppEnv               string
	LogLevel             string
	TursoDatabaseURL     string
	TursoAuthToken       string
	BotToken             string
	AdminID              int64
	MaxNodes             int
	ConcurrencyWorkers   int
	TestTimeout          time.Duration
	SublistPath          string
	BlacklistPath        string
	OutputDir            string
	DryRun               bool
	CDNHost              string
	SNIHost              string
	EnableExtendedProbes bool
	Endpoints            []string
}

// LoadConfig loads configuration from environment variables and CLI flags.
func LoadConfig() (*Config, error) {
	// Attempt to load .env file if it exists, ignore if not found
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:               getEnvOrDefault("APP_ENV", "development"),
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
		TursoDatabaseURL:     os.Getenv("TURSO_DATABASE_URL"),
		TursoAuthToken:       os.Getenv("TURSO_AUTH_TOKEN"),
		BotToken:             os.Getenv("BOT_TOKEN"),
		AdminID:              getEnvAsInt64("ADMIN_ID", 0),
		MaxNodes:             getEnvAsInt("MAX_NODES", 500),
		ConcurrencyWorkers:   getEnvAsInt("CONCURRENCY_WORKERS", 100),
		TestTimeout:          time.Duration(getEnvAsInt("TEST_TIMEOUT_SECONDS", 5)) * time.Second,
		SublistPath:          getEnvOrDefault("SUBLIST_PATH", "./resources/sublist.json"),
		BlacklistPath:        getEnvOrDefault("BLACKLIST_PATH", "./blacklist.txt"),
		OutputDir:            getEnvOrDefault("OUTPUT_DIR", "./result"),
		DryRun:               getEnvAsBool("DRY_RUN", false),
		CDNHost:              getEnvOrDefault("CDN_HOST", "104.18.2.2"),
		SNIHost:              getEnvOrDefault("SNI_HOST", "meet.google.com"),
		EnableExtendedProbes: getEnvAsBool("ENABLE_EXTENDED_PROBES", false),
		Endpoints:            []string{"https://myip.ipeek.workers.dev"},
	}

	// Parse flags if flagset is not already parsed
	if !flag.Parsed() {
		flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
		flag.IntVar(&cfg.MaxNodes, "max-nodes", cfg.MaxNodes, "Maximum number of verified nodes to save")
		flag.IntVar(&cfg.ConcurrencyWorkers, "concurrency", cfg.ConcurrencyWorkers, "Number of concurrent test workers")
		flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Run scraper and tests without committing to Turso DB")
		flag.StringVar(&cfg.SublistPath, "sublist", cfg.SublistPath, "Path to sublist.json file")
		flag.StringVar(&cfg.BlacklistPath, "blacklist", cfg.BlacklistPath, "Path to blacklist.txt file")
		flag.Parse()
	}

	return cfg, nil
}

// IsProduction returns true if running in production environment.
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.AppEnv) == "production" || strings.ToLower(c.AppEnv) == "prod"
}

// HasDatabase returns true if Turso database credentials are provided.
func (c *Config) HasDatabase() bool {
	return c.TursoDatabaseURL != ""
}

// HasTelegram returns true if Telegram bot token and admin ID are provided.
func (c *Config) HasTelegram() bool {
	return c.BotToken != "" && c.AdminID != 0
}

func getEnvOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valStr := os.Getenv(key)
	if val, err := strconv.Atoi(strings.TrimSpace(valStr)); err == nil {
		return val
	}
	return fallback
}

func getEnvAsInt64(key string, fallback int64) int64 {
	valStr := os.Getenv(key)
	if val, err := strconv.ParseInt(strings.TrimSpace(valStr), 10, 64); err == nil {
		return val
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	valStr := os.Getenv(key)
	if val, err := strconv.ParseBool(strings.TrimSpace(valStr)); err == nil {
		return val
	}
	return fallback
}
