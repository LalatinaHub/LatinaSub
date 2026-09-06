package notifier

import (
	"context"
	"time"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
)

// NoopNotifier provides a silent / logging-only implementation of Notifier.
type NoopNotifier struct{}

// NewNoop creates a new NoopNotifier instance.
func NewNoop() *NoopNotifier {
	return &NoopNotifier{}
}

func (n *NoopNotifier) SendStartup(totalSources int, maxNodes int) error {
	logger.Info().
		Int("total_sources", totalSources).
		Int("max_nodes", maxNodes).
		Msg("[Notifier:Noop] Engine startup")
	return nil
}

func (n *NoopNotifier) SendHeartbeat(stats ProgressStats) error {
	logger.Debug().
		Int("tested", stats.TestedNodes).
		Int("total", stats.TotalNodes).
		Int("active", stats.ActiveNodes).
		Msg("[Notifier:Noop] Heartbeat progress")
	return nil
}

func (n *NoopNotifier) SendSummary(summary RunSummary) error {
	logger.Info().
		Dur("duration", summary.Duration).
		Int("scraped", summary.TotalScraped).
		Int("active", summary.ActiveSaved).
		Int("dead", summary.DeadDetected).
		Msg("[Notifier:Noop] Run finished summary")
	return nil
}

func (n *NoopNotifier) SendDocument(filename string, data []byte, caption string) error {
	logger.Info().
		Str("filename", filename).
		Int("bytes", len(data)).
		Str("caption", caption).
		Msg("[Notifier:Noop] Send document")
	return nil
}

func (n *NoopNotifier) SendError(err error, context string) error {
	logger.Error().
		Err(err).
		Str("context", context).
		Msg("[Notifier:Noop] Error notification")
	return nil
}

func (n *NoopNotifier) StartHeartbeat(ctx context.Context, interval time.Duration, getStats func() ProgressStats) func() {
	return func() {}
}
