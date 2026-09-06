package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("APP_ENV")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("TURSO_DATABASE_URL")
	os.Unsetenv("TURSO_AUTH_TOKEN")
	os.Unsetenv("BOT_TOKEN")
	os.Unsetenv("ADMIN_ID")
	os.Unsetenv("MAX_NODES")
	os.Unsetenv("CONCURRENCY_WORKERS")
	os.Unsetenv("TEST_TIMEOUT_SECONDS")
	os.Unsetenv("DRY_RUN")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 500, cfg.MaxNodes)
	assert.Equal(t, 100, cfg.ConcurrencyWorkers)
	assert.Equal(t, 5*time.Second, cfg.TestTimeout)
	assert.False(t, cfg.DryRun)
	assert.False(t, cfg.IsProduction())
	assert.False(t, cfg.HasDatabase())
	assert.False(t, cfg.HasTelegram())
}

func TestLoadConfigCustomEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("TURSO_DATABASE_URL", "libsql://test.turso.io")
	t.Setenv("TURSO_AUTH_TOKEN", "secret-token")
	t.Setenv("BOT_TOKEN", "1234:telegram")
	t.Setenv("ADMIN_ID", "987654")
	t.Setenv("MAX_NODES", "200")
	t.Setenv("CONCURRENCY_WORKERS", "50")
	t.Setenv("TEST_TIMEOUT_SECONDS", "10")
	t.Setenv("DRY_RUN", "true")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "production", cfg.AppEnv)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "libsql://test.turso.io", cfg.TursoDatabaseURL)
	assert.Equal(t, "secret-token", cfg.TursoAuthToken)
	assert.Equal(t, "1234:telegram", cfg.BotToken)
	assert.Equal(t, int64(987654), cfg.AdminID)
	assert.Equal(t, 200, cfg.MaxNodes)
	assert.Equal(t, 50, cfg.ConcurrencyWorkers)
	assert.Equal(t, 10*time.Second, cfg.TestTimeout)
	assert.True(t, cfg.DryRun)
	assert.True(t, cfg.IsProduction())
	assert.True(t, cfg.HasDatabase())
	assert.True(t, cfg.HasTelegram())
}
