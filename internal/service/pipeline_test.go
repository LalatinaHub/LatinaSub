package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LalatinaHub/LatinaSub/config"
	"github.com/LalatinaHub/LatinaSub/internal/blacklist"
	"github.com/LalatinaHub/LatinaSub/internal/notifier"
	"github.com/LalatinaHub/LatinaSub/internal/provider"
	"github.com/LalatinaHub/LatinaSub/internal/sandbox"
	"github.com/LalatinaHub/LatinaSub/internal/storage"
	"github.com/LalatinaHub/common/probe"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBoxRunner implements sandbox.BoxRunner for unit and integration testing.
type MockBoxRunner struct {
	failServers map[string]bool
}

func (m *MockBoxRunner) Run(
	ctx context.Context,
	opt option.Options,
	listenPort int,
	endpoints []string,
	enableExtendedProbes bool,
) (sandbox.GeoIPResult, int64, *probe.Result, *probe.Result, error) {
	if len(opt.Outbounds) > 0 {
		tag := strings.ToLower(opt.Outbounds[0].Tag)
		if strings.Contains(tag, "dead") {
			return sandbox.GeoIPResult{}, 0, nil, nil, fmt.Errorf("connection refused to %s", tag)
		}
	}

	geo := sandbox.GeoIPResult{
		IP:             "103.10.10.1",
		Country:        "ID",
		AsOrganization: "Biznet Networks",
	}

	return geo, 35, nil, nil, nil
}

func TestPipeline_Execute_DryRun(t *testing.T) {
	tempDir := t.TempDir()
	blacklistFile := filepath.Join(tempDir, "blacklist.txt")
	resultDir := filepath.Join(tempDir, "result")

	// 1. Setup mock subscription server
	feedContent := `vmess://eyJhZGQiOiJsaXZlLnNlcnZlci5jb20iLCJhaWQiOiIwIiwiaG9zdCI6ImxpdmUuc2VydmVyLmNvbSIsImlkIjoiMDAwMCIsIm5ldCI6IndzIiwicGF0aCI6Ii93cyIsInBvcnQiOiI0NDMiLCJwcyI6IlRlc3RWbWVzcyIsInNuaSI6ImxpdmUuc2VydmVyLmNvbSIsInRscyI6InRscyIsInYiOiIyIn0=
vless://1111@dead.server.com:443?security=tls&type=ws&host=dead.server.com#DeadVless
trojan://pass@trojan.server.com:443?security=tls&type=ws&host=trojan.server.com#TestTrojan
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(feedContent))
	}))
	defer server.Close()

	// 2. Create mock sublist.json
	sublistPath := filepath.Join(tempDir, "sublist.json")
	sublist := []provider.SubscriptionSource{
		{
			ID:      1,
			Remarks: "Mock Feed",
			URL:     server.URL,
			Enabled: true,
		},
	}
	sublistBytes, err := json.Marshal(sublist)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(sublistPath, sublistBytes, 0644))

	// 3. Setup pipeline dependencies
	cfg := &config.Config{
		AppEnv:             "test",
		MaxNodes:           10,
		ConcurrencyWorkers: 4,
		TestTimeout:        2 * time.Second,
		SublistPath:        sublistPath,
		BlacklistPath:      blacklistFile,
		DryRun:             true,
	}

	prov := provider.New(provider.WithWorkers(2))
	bl := blacklist.New()
	require.NoError(t, bl.Save(blacklistFile)) // create empty blacklist file

	sbCfg := sandbox.DefaultConfig()
	sbCfg.Concurrency = 2
	tester := sandbox.NewTester(sbCfg, bl)
	mockRunner := &MockBoxRunner{
		failServers: map[string]bool{
			"dead.server.com": true,
		},
	}
	tester.SetRunner(mockRunner)

	exporter := storage.NewExporter(resultDir)
	notif := notifier.NewNoop()

	pipeline := NewPipeline(cfg, prov, bl, tester, nil, exporter, notif)

	// 4. Run pipeline
	ctx := context.Background()
	summary, err := pipeline.Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// 5. Assert summary metrics
	assert.Equal(t, 1, summary.TotalSources)
	assert.Equal(t, 3, summary.TotalScraped)
	assert.Equal(t, 3, summary.UniqueNodes)
	assert.Equal(t, 6, summary.ActiveSaved)  // 2 live nodes * 3 connection modes (cdn, sni, direct)
	assert.Equal(t, 1, summary.DeadDetected) // dead.server.com
	assert.False(t, summary.DatabaseSynced, "database should not be synced in dry-run mode")
	assert.True(t, summary.FilesExported, "local files should be exported")
	assert.Equal(t, 6, summary.TopCountries["ID"])

	// 6. Verify exported files exist and are valid
	assert.FileExists(t, filepath.Join(resultDir, "nodes"))
	assert.FileExists(t, filepath.Join(resultDir, "sub"))
	assert.FileExists(t, filepath.Join(resultDir, "singbox.json"))
	assert.FileExists(t, filepath.Join(resultDir, "clash.yaml"))

	nodesContent, err := os.ReadFile(filepath.Join(resultDir, "nodes"))
	require.NoError(t, err)
	assert.Contains(t, string(nodesContent), "eyJhZGQiOiJsaXZlLnNlcnZlci5jb20i")
	assert.Contains(t, string(nodesContent), "trojan.server.com")
	assert.NotContains(t, string(nodesContent), "dead.server.com")

	// 7. Verify blacklist was persisted to disk
	assert.FileExists(t, blacklistFile)
	loadedBL := blacklist.New()
	count, err := loadedBL.Load(blacklistFile)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "blacklist should contain exactly 1 dead node")
}

func TestPipeline_Execute_WithDatabase(t *testing.T) {
	tempDir := t.TempDir()
	blacklistFile := filepath.Join(tempDir, "blacklist.txt")
	resultDir := filepath.Join(tempDir, "result")

	feedContent := `trojan://pass@active.server.com:443?security=tls&type=ws&host=active.server.com#ActiveTrojan`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(feedContent))
	}))
	defer server.Close()

	sublistPath := filepath.Join(tempDir, "sublist.json")
	sublist := []provider.SubscriptionSource{
		{ID: 1, Remarks: "Active Feed", URL: server.URL, Enabled: true},
	}
	subBytes, err := json.Marshal(sublist)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(sublistPath, subBytes, 0644))

	cfg := &config.Config{
		AppEnv:             "test",
		MaxNodes:           10,
		ConcurrencyWorkers: 2,
		TestTimeout:        2 * time.Second,
		SublistPath:        sublistPath,
		BlacklistPath:      blacklistFile,
		DryRun:             false,
		TursoDatabaseURL:   "libsql://test.turso.io",
	}

	prov := provider.New(provider.WithWorkers(2))
	bl := blacklist.New()
	sbCfg := sandbox.DefaultConfig()
	tester := sandbox.NewTester(sbCfg, bl)
	tester.SetRunner(&MockBoxRunner{})

	exporter := storage.NewExporter(resultDir)
	notif := notifier.NewNoop()

	// Mock database
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	dbStorage, err := storage.NewDatabase(mockDB)
	require.NoError(t, err)

	// Expectations for InitSchema
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS proxies")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE INDEX IF NOT EXISTS idx_proxies_filter")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE INDEX IF NOT EXISTS idx_proxies_country")).WillReturnResult(sqlmock.NewResult(0, 0))

	// Expectations for SaveBatch (1 candidate node verified for 3 modes: cdn, sni, direct => 3 distinct accounts)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM proxies;")).WillReturnResult(sqlmock.NewResult(0, 0))
	prep := mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO proxies"))
	prep.ExpectExec().WillReturnResult(sqlmock.NewResult(1, 1))
	prep.ExpectExec().WillReturnResult(sqlmock.NewResult(2, 1))
	prep.ExpectExec().WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()

	pipeline := NewPipeline(cfg, prov, bl, tester, dbStorage, exporter, notif)

	summary, err := pipeline.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.True(t, summary.DatabaseSynced, "database should be marked synced")
	assert.Equal(t, 3, summary.ActiveSaved)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func BenchmarkPipeline_LargeBatchProcessing(b *testing.B) {
	// Benchmark memory allocations when parsing and deduplicating large sets of nodes
	rawLines := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		rawLines[i] = fmt.Sprintf("trojan://pass%d@node%d.example.com:443?security=tls&type=ws&host=node%d.example.com#Node-%d", i, i, i, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		nodes := provider.ParseNodes(rawLines)
		deduped := provider.DeduplicateNodes(nodes)
		if len(deduped) == 0 {
			b.Fatal("unexpected empty deduped slice")
		}
	}
}
