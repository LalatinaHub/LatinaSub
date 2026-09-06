package notifier

import (
	"context"
	"time"
)

// ProgressStats captures live telemetry of the scraping and verification cycle.
type ProgressStats struct {
	ScrapedSources int           `json:"scraped_sources"`
	TotalSources   int           `json:"total_sources"`
	TestedNodes    int           `json:"tested_nodes"`
	TotalNodes     int           `json:"total_nodes"`
	ActiveNodes    int           `json:"active_nodes"`
	MaxNodes       int           `json:"max_nodes"`
	BlacklistCount int           `json:"blacklist_count"`
	Duration       time.Duration `json:"duration"`
}

// RunSummary represents the comprehensive operational report after pipeline completion.
type RunSummary struct {
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	Duration        time.Duration     `json:"duration"`
	TotalSources    int               `json:"total_sources"`
	TotalScraped    int               `json:"total_scraped"`
	UniqueNodes     int               `json:"unique_nodes"`
	BlacklistedInit int               `json:"blacklisted_init"`
	BlacklistedEnd  int               `json:"blacklisted_end"`
	DeadDetected    int               `json:"dead_detected"`
	TestedNodes     int               `json:"tested_nodes"`
	ActiveSaved     int               `json:"active_saved"`
	DatabaseSynced  bool              `json:"database_synced"`
	FilesExported   bool              `json:"files_exported"`
	TopCountries    map[string]int    `json:"top_countries,omitempty"`
	TopProtocols    map[string]int    `json:"top_protocols,omitempty"`
}

// Notifier defines the communication and telemetry contract for alerts and reports.
type Notifier interface {
	SendStartup(totalSources int, maxNodes int) error
	SendHeartbeat(stats ProgressStats) error
	SendSummary(summary RunSummary) error
	SendDocument(filename string, data []byte, caption string) error
	SendError(err error, context string) error
	StartHeartbeat(ctx context.Context, interval time.Duration, getStats func() ProgressStats) (stop func())
}
