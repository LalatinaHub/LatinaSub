package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LalatinaHub/LatinaSub/config"
	"github.com/LalatinaHub/LatinaSub/internal/blacklist"
	"github.com/LalatinaHub/LatinaSub/internal/notifier"
	"github.com/LalatinaHub/LatinaSub/internal/provider"
	"github.com/LalatinaHub/LatinaSub/internal/sandbox"
	"github.com/LalatinaHub/LatinaSub/internal/service"
	"github.com/LalatinaHub/LatinaSub/internal/storage"
	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/common/database"
)

func main() {
	// 1. Load application configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load application configuration")
	}

	// 2. Initialize structured zerolog logger
	logger.SetupLogger(cfg.LogLevel, cfg.IsProduction())
	logger.Info().
		Str("app_env", cfg.AppEnv).
		Int("max_nodes", cfg.MaxNodes).
		Int("concurrency", cfg.ConcurrencyWorkers).
		Bool("dry_run", cfg.DryRun).
		Msg("Starting LatinaSub Verification & Scraping Engine...")

	// 3. Set up root context with OS interrupt signal trap for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		logger.Warn().Str("signal", sig.String()).Msg("Interrupt signal received, shutting down gracefully...")
		cancel()
	}()

	// 4. Initialize blacklist subsystem
	bl := blacklist.New()
	if loaded, err := bl.Load(cfg.BlacklistPath); err != nil {
		logger.Warn().Err(err).Str("path", cfg.BlacklistPath).Msg("Unable to load blacklist file, starting with empty blacklist")
	} else {
		logger.Info().Int("total_hashes", loaded).Str("path", cfg.BlacklistPath).Msg("Loaded dead account blacklist successfully")
	}

	// 5. Initialize provider subsystem
	prov := provider.New(
		provider.WithWorkers(cfg.ConcurrencyWorkers),
		provider.WithTimeout(30 * time.Second),
	)

	// 6. Initialize sandbox tester subsystem
	testerCfg := sandbox.Config{
		CDNHost:              cfg.CDNHost,
		SNIHost:              cfg.SNIHost,
		Concurrency:          cfg.ConcurrencyWorkers,
		Timeout:              cfg.TestTimeout,
		Endpoints:            cfg.Endpoints,
		EnableExtendedProbes: cfg.EnableExtendedProbes,
		MaxNodes:             cfg.MaxNodes,
	}
	tester := sandbox.NewTester(testerCfg, bl)

	// 7. Initialize storage subsystems (File Exporter & Turso Database)
	exporter := storage.NewExporter(cfg.OutputDir)

	var dbRepo *storage.Database
	if cfg.HasDatabase() && !cfg.DryRun {
		logger.Info().Msg("Initializing Turso LibSQL database pool...")
		sqlDB, err := database.GetDB()
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Turso database, continuing in local-only mode")
		} else if sqlDB != nil {
			dbRepo, err = storage.NewDatabase(sqlDB)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to initialize database repository")
			} else {
				logger.Info().Msg("Connected to Turso LibSQL database successfully")
			}
		}
	} else if cfg.DryRun {
		logger.Info().Msg("Dry run mode active: skipping Turso remote database connection")
	} else {
		logger.Warn().Msg("TURSO_DATABASE_URL not set: running in standalone local mode")
	}

	defer func() {
		if cfg.HasDatabase() && !cfg.DryRun {
			logger.Info().Msg("Closing Turso database pool...")
			_ = database.Close()
		}
		logger.Info().Msg("LatinaSub engine shutdown complete")
	}()

	// 8. Initialize notifier subsystem
	notif := notifier.New(cfg.BotToken, cfg.AdminID)

	// 9. Construct and execute the end-to-end pipeline
	pipeline := service.NewPipeline(cfg, prov, bl, tester, dbRepo, exporter, notif)
	summary, err := pipeline.Execute(ctx)
	if err != nil {
		logger.Fatal().Err(err).Msg("LatinaSub pipeline execution failed")
	}

	logger.Info().
		Int("active_saved", summary.ActiveSaved).
		Int("dead_detected", summary.DeadDetected).
		Dur("duration", summary.Duration).
		Msg("LatinaSub pipeline completed all phases successfully")
}
