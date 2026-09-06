package notifier

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NicoNex/echotron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTelegramClient struct {
	mu            sync.Mutex
	sentMessages  []string
	sentDocuments []string
	shouldFail    bool
}

func (m *mockTelegramClient) SendMessage(text string, chatID int64, opts *echotron.MessageOptions) (echotron.APIResponseMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return echotron.APIResponseMessage{}, errors.New("telegram api error")
	}

	m.sentMessages = append(m.sentMessages, text)
	return echotron.APIResponseMessage{
		APIResponseBase: echotron.APIResponseBase{Ok: true},
	}, nil
}

func (m *mockTelegramClient) SendDocument(file echotron.InputFile, chatID int64, opts *echotron.DocumentOptions) (echotron.APIResponseMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return echotron.APIResponseMessage{}, errors.New("telegram api error")
	}

	caption := ""
	if opts != nil {
		caption = opts.Caption
	}
	m.sentDocuments = append(m.sentDocuments, caption)
	return echotron.APIResponseMessage{
		APIResponseBase: echotron.APIResponseBase{Ok: true},
	}, nil
}

func TestNew_GracefulFallback(t *testing.T) {
	t.Run("Empty token returns NoopNotifier", func(t *testing.T) {
		n := New("", 123456)
		_, ok := n.(*NoopNotifier)
		assert.True(t, ok)
	})

	t.Run("Whitespace token returns NoopNotifier", func(t *testing.T) {
		n := New("   ", 123456)
		_, ok := n.(*NoopNotifier)
		assert.True(t, ok)
	})

	t.Run("Zero or negative adminID returns NoopNotifier", func(t *testing.T) {
		n1 := New("valid:token", 0)
		_, ok1 := n1.(*NoopNotifier)
		assert.True(t, ok1)

		n2 := New("valid:token", -99)
		_, ok2 := n2.(*NoopNotifier)
		assert.True(t, ok2)
	})

	t.Run("Valid credentials returns TelegramNotifier", func(t *testing.T) {
		n := New("123456:ABC-DEF", 987654)
		tg, ok := n.(*TelegramNotifier)
		assert.True(t, ok)
		assert.Equal(t, int64(987654), tg.adminID)
	})
}

func TestNoopNotifier(t *testing.T) {
	n := NewNoop()

	assert.NoError(t, n.SendStartup(10, 500))
	assert.NoError(t, n.SendHeartbeat(ProgressStats{}))
	assert.NoError(t, n.SendSummary(RunSummary{}))
	assert.NoError(t, n.SendDocument("test.txt", []byte("hello"), "caption"))
	assert.NoError(t, n.SendError(errors.New("test error"), "test context"))

	stop := n.StartHeartbeat(context.Background(), 10*time.Millisecond, nil)
	assert.NotNil(t, stop)
	stop()
}

func TestTelegramNotifier_Messages(t *testing.T) {
	mock := &mockTelegramClient{}
	notifier := NewWithClient(mock, 12345678)

	t.Run("SendStartup message format", func(t *testing.T) {
		err := notifier.SendStartup(15, 200)
		require.NoError(t, err)

		mock.mu.Lock()
		defer mock.mu.Unlock()
		require.Len(t, mock.sentMessages, 1)
		msg := mock.sentMessages[0]
		assert.Contains(t, msg, "LatinaSub Engine Started")
		assert.Contains(t, msg, "15 subscription endpoints")
		assert.Contains(t, msg, "200 verified accounts")
	})

	t.Run("SendHeartbeat message format", func(t *testing.T) {
		stats := ProgressStats{
			TestedNodes:    50,
			TotalNodes:     100,
			ActiveNodes:    25,
			MaxNodes:       200,
			BlacklistCount: 25,
			Duration:       90 * time.Second,
		}

		err := notifier.SendHeartbeat(stats)
		require.NoError(t, err)

		mock.mu.Lock()
		defer mock.mu.Unlock()
		require.Len(t, mock.sentMessages, 2)
		msg := mock.sentMessages[1]
		assert.Contains(t, msg, "Progress Heartbeat")
		assert.Contains(t, msg, "50 / 100 (50.0%)")
		assert.Contains(t, msg, "25 / 200")
		assert.Contains(t, msg, "Blacklisted Dead</b>: 25")
	})

	t.Run("SendSummary message format with top countries", func(t *testing.T) {
		summary := RunSummary{
			Duration:       2*time.Minute + 15*time.Second,
			TotalScraped:   1500,
			UniqueNodes:    800,
			ActiveSaved:    150,
			DeadDetected:   650,
			DatabaseSynced: true,
			FilesExported:  true,
			TopCountries: map[string]int{
				"ID": 50,
				"SG": 40,
				"US": 30,
				"JP": 20,
				"DE": 10,
			},
		}

		err := notifier.SendSummary(summary)
		require.NoError(t, err)

		mock.mu.Lock()
		defer mock.mu.Unlock()
		require.Len(t, mock.sentMessages, 3)
		msg := mock.sentMessages[2]
		assert.Contains(t, msg, "Pipeline Completed")
		assert.Contains(t, msg, "1500 proxies")
		assert.Contains(t, msg, "800 proxies")
		assert.Contains(t, msg, "150")
		assert.Contains(t, msg, "650")
		assert.Contains(t, msg, "✅ Committed")
		assert.Contains(t, msg, "✅ Exported")
		assert.Contains(t, msg, "ID (50)")
		assert.Contains(t, msg, "SG (40)")
	})

	t.Run("SendDocument sends byte attachment", func(t *testing.T) {
		err := notifier.SendDocument("summary.txt", []byte("test summary content"), "Execution Summary")
		require.NoError(t, err)

		mock.mu.Lock()
		defer mock.mu.Unlock()
		require.Len(t, mock.sentDocuments, 1)
		assert.Equal(t, "Execution Summary", mock.sentDocuments[0])
	})

	t.Run("SendError alert format", func(t *testing.T) {
		err := notifier.SendError(errors.New("connection failed"), "Database Connection")
		require.NoError(t, err)

		// nil error is a noop
		assert.NoError(t, notifier.SendError(nil, "Ignored"))

		mock.mu.Lock()
		defer mock.mu.Unlock()
		require.Len(t, mock.sentMessages, 4)
		msg := mock.sentMessages[3]
		assert.Contains(t, msg, "LatinaSub Alert")
		assert.Contains(t, msg, "Database Connection")
		assert.Contains(t, msg, "connection failed")
	})

	t.Run("Client error propagation", func(t *testing.T) {
		failingMock := &mockTelegramClient{shouldFail: true}
		failingNotifier := NewWithClient(failingMock, 12345)

		assert.Error(t, failingNotifier.SendStartup(1, 1))
		assert.Error(t, failingNotifier.SendHeartbeat(ProgressStats{}))
		assert.Error(t, failingNotifier.SendSummary(RunSummary{}))
		assert.Error(t, failingNotifier.SendDocument("doc.txt", []byte("a"), "cap"))
		assert.Error(t, failingNotifier.SendError(errors.New("boom"), "ctx"))
	})
}

func TestTelegramNotifier_StartHeartbeat(t *testing.T) {
	mock := &mockTelegramClient{}
	notifier := NewWithClient(mock, 12345678)

	var callCount int64
	getStats := func() ProgressStats {
		val := atomic.AddInt64(&callCount, 1)
		return ProgressStats{
			TestedNodes: int(val),
			TotalNodes:  10,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Short interval for testing
	stop := notifier.StartHeartbeat(ctx, 25*time.Millisecond, getStats)
	time.Sleep(90 * time.Millisecond)
	stop()

	assert.GreaterOrEqual(t, atomic.LoadInt64(&callCount), int64(2))

	mock.mu.Lock()
	defer mock.mu.Unlock()
	assert.GreaterOrEqual(t, len(mock.sentMessages), 2)
}
