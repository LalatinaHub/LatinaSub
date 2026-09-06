package notifier

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/NicoNex/echotron/v3"
)

// TelegramClient abstracts the underlying echotron API calls for testability.
type TelegramClient interface {
	SendMessage(text string, chatID int64, opts *echotron.MessageOptions) (echotron.APIResponseMessage, error)
	SendDocument(file echotron.InputFile, chatID int64, opts *echotron.DocumentOptions) (echotron.APIResponseMessage, error)
}

type echotronClientWrapper struct {
	api echotron.API
}

func (w echotronClientWrapper) SendMessage(text string, chatID int64, opts *echotron.MessageOptions) (echotron.APIResponseMessage, error) {
	return w.api.SendMessage(text, chatID, opts)
}

func (w echotronClientWrapper) SendDocument(file echotron.InputFile, chatID int64, opts *echotron.DocumentOptions) (echotron.APIResponseMessage, error) {
	return w.api.SendDocument(file, chatID, opts)
}

// TelegramNotifier sends operational status, heartbeats, and reports via Telegram Bot API.
type TelegramNotifier struct {
	client  TelegramClient
	adminID int64
}

// New constructs a Notifier instance.
// If token is empty or adminID is invalid (<= 0), it gracefully returns a NoopNotifier.
func New(token string, adminID int64) Notifier {
	cleanToken := strings.TrimSpace(token)
	if cleanToken == "" || adminID <= 0 {
		logger.Info().
			Msg("Telegram notifier credentials not set or invalid; running with NoopNotifier")
		return NewNoop()
	}

	return &TelegramNotifier{
		client:  echotronClientWrapper{api: echotron.NewAPI(cleanToken)},
		adminID: adminID,
	}
}

// NewWithClient allows injecting a custom or mock TelegramClient (useful for unit testing).
func NewWithClient(client TelegramClient, adminID int64) *TelegramNotifier {
	return &TelegramNotifier{
		client:  client,
		adminID: adminID,
	}
}

func (t *TelegramNotifier) SendStartup(totalSources int, maxNodes int) error {
	msg := fmt.Sprintf(
		"🚀 <b>LatinaSub Engine Started</b>\n"+
			"• <b>Sources</b>: %d subscription endpoints\n"+
			"• <b>Target Limit</b>: %d verified accounts\n"+
			"• <b>Timestamp</b>: %s\n"+
			"• <b>Status</b>: Scraping & sandbox verification running...",
		totalSources,
		maxNodes,
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)

	opts := &echotron.MessageOptions{
		ParseMode: echotron.HTML,
	}

	_, err := t.client.SendMessage(msg, t.adminID, opts)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to send Telegram startup message")
		return err
	}
	return nil
}

func (t *TelegramNotifier) SendHeartbeat(stats ProgressStats) error {
	var percent float64
	if stats.TotalNodes > 0 {
		percent = (float64(stats.TestedNodes) / float64(stats.TotalNodes)) * 100.0
	}

	durationStr := stats.Duration.Round(time.Second).String()

	msg := fmt.Sprintf(
		"💓 <b>LatinaSub Progress Heartbeat</b>\n"+
			"• <b>Progress</b>: %d / %d (%.1f%%)\n"+
			"• <b>Active Found</b>: %d / %d\n"+
			"• <b>Blacklisted Dead</b>: %d\n"+
			"• <b>Elapsed Time</b>: %s",
		stats.TestedNodes,
		stats.TotalNodes,
		percent,
		stats.ActiveNodes,
		stats.MaxNodes,
		stats.BlacklistCount,
		durationStr,
	)

	opts := &echotron.MessageOptions{
		ParseMode: echotron.HTML,
	}

	_, err := t.client.SendMessage(msg, t.adminID, opts)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to send Telegram heartbeat message")
		return err
	}
	return nil
}

func (t *TelegramNotifier) SendSummary(summary RunSummary) error {
	dbStatus := "❌ Failed / Skipped"
	if summary.DatabaseSynced {
		dbStatus = "✅ Committed"
	}

	filesStatus := "❌ Skipped"
	if summary.FilesExported {
		filesStatus = "✅ Exported"
	}

	var countrySummary strings.Builder
	if len(summary.TopCountries) > 0 {
		countrySummary.WriteString("\n• <b>Top Countries</b>: ")
		type ccCount struct {
			cc    string
			count int
		}
		var list []ccCount
		for cc, count := range summary.TopCountries {
			list = append(list, ccCount{cc: cc, count: count})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].count > list[j].count
		})

		limit := 5
		if len(list) < limit {
			limit = len(list)
		}
		var ccParts []string
		for i := 0; i < limit; i++ {
			ccParts = append(ccParts, fmt.Sprintf("%s (%d)", list[i].cc, list[i].count))
		}
		countrySummary.WriteString(strings.Join(ccParts, ", "))
	}

	msg := fmt.Sprintf(
		"🏁 <b>LatinaSub Pipeline Completed</b>\n"+
			"• <b>Duration</b>: %s\n"+
			"• <b>Scraped Raw</b>: %d proxies\n"+
			"• <b>Unique Candidates</b>: %d proxies\n"+
			"• <b>Active Accounts Saved</b>: %d\n"+
			"• <b>Dead Nodes Detected</b>: %d\n"+
			"• <b>Database Sync</b>: %s\n"+
			"• <b>File Subscriptions</b>: %s%s",
		summary.Duration.Round(time.Second).String(),
		summary.TotalScraped,
		summary.UniqueNodes,
		summary.ActiveSaved,
		summary.DeadDetected,
		dbStatus,
		filesStatus,
		countrySummary.String(),
	)

	opts := &echotron.MessageOptions{
		ParseMode: echotron.HTML,
	}

	_, err := t.client.SendMessage(msg, t.adminID, opts)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to send Telegram summary message")
		return err
	}
	return nil
}

func (t *TelegramNotifier) SendDocument(filename string, data []byte, caption string) error {
	file := echotron.NewInputFileBytes(filename, data)
	opts := &echotron.DocumentOptions{
		Caption: caption,
	}

	_, err := t.client.SendDocument(file, t.adminID, opts)
	if err != nil {
		logger.Warn().Err(err).Str("filename", filename).Msg("Failed to send Telegram document attachment")
		return err
	}
	return nil
}

func (t *TelegramNotifier) SendError(err error, context string) error {
	if err == nil {
		return nil
	}

	msg := fmt.Sprintf(
		"⚠️ <b>LatinaSub Alert: Error Encountered</b>\n"+
			"• <b>Context</b>: %s\n"+
			"• <b>Error</b>: <code>%s</code>\n"+
			"• <b>Timestamp</b>: %s",
		context,
		err.Error(),
		time.Now().Format("2006-01-02 15:04:05 MST"),
	)

	opts := &echotron.MessageOptions{
		ParseMode: echotron.HTML,
	}

	_, sendErr := t.client.SendMessage(msg, t.adminID, opts)
	if sendErr != nil {
		logger.Warn().Err(sendErr).Msg("Failed to send Telegram error alert")
		return sendErr
	}
	return nil
}

// StartHeartbeat launches a background ticker that periodically sends progress stats to admin.
// It returns a stop function that cleanly stops the ticker.
func (t *TelegramNotifier) StartHeartbeat(ctx context.Context, interval time.Duration, getStats func() ProgressStats) func() {
	if interval <= 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				if getStats != nil {
					stats := getStats()
					_ = t.SendHeartbeat(stats)
				}
			}
		}
	}()

	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}
