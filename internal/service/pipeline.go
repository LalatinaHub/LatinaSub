package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/LalatinaHub/LatinaSub/config"
	"github.com/LalatinaHub/LatinaSub/internal/blacklist"
	"github.com/LalatinaHub/LatinaSub/internal/notifier"
	"github.com/LalatinaHub/LatinaSub/internal/provider"
	"github.com/LalatinaHub/LatinaSub/internal/sandbox"
	"github.com/LalatinaHub/LatinaSub/internal/storage"
	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/proxy"
)

// Pipeline coordinates the end-to-end proxy lifecycle: scraping, blacklisting, sandbox probing,
// persistence, and reporting.
type Pipeline struct {
	cfg      *config.Config
	prov     *provider.Provider
	bl       *blacklist.Blacklist
	tester   *sandbox.Tester
	db       *storage.Database
	exporter *storage.Exporter
	notif    notifier.Notifier

	// Telemetry atomics
	scrapedSources int64
	testedNodes    int64
	totalNodes     int64
	activeNodes    int64
}

// NewPipeline constructs a new Pipeline with all required subsystems.
func NewPipeline(
	cfg *config.Config,
	prov *provider.Provider,
	bl *blacklist.Blacklist,
	tester *sandbox.Tester,
	db *storage.Database,
	exporter *storage.Exporter,
	notif notifier.Notifier,
) *Pipeline {
	if notif == nil {
		notif = notifier.NewNoop()
	}
	if exporter == nil {
		exporter = storage.NewExporter("./result")
	}
	if tester != nil && cfg != nil && cfg.MaxNodes > 0 {
		tester.SetMaxNodes(cfg.MaxNodes)
	}

	return &Pipeline{
		cfg:      cfg,
		prov:     prov,
		bl:       bl,
		tester:   tester,
		db:       db,
		exporter: exporter,
		notif:    notif,
	}
}

// Execute orchestrates the full scraping, testing, saving, and reporting cycle.
func (p *Pipeline) Execute(ctx context.Context) (*notifier.RunSummary, error) {
	startTime := time.Now()
	logger.Info().Msg("=== Starting LatinaSub Pipeline Execution ===")

	// 1. Load subscription sources
	sources, err := p.prov.LoadSublistFromFile(p.cfg.SublistPath)
	if err != nil || len(sources) == 0 {
		logger.Warn().
			Err(err).
			Str("path", p.cfg.SublistPath).
			Msg("Local sublist unavailable or empty; using default fallback sources")
		sources = []provider.SubscriptionSource{
			{URL: "https://raw.githubusercontent.com/LalatinaHub/Mineral/master/result/nodes"},
			{URL: "https://raw.githubusercontent.com/barry-far/V2ray-Configs/main/All_Configs_Sub.txt"},
		}
	}

	totalSources := len(sources)
	p.notif.SendStartup(totalSources, p.cfg.MaxNodes)

	// 2. Start background heartbeat ticker
	stopHeartbeat := p.notif.StartHeartbeat(ctx, 60*time.Second, func() notifier.ProgressStats {
		return notifier.ProgressStats{
			ScrapedSources: int(atomic.LoadInt64(&p.scrapedSources)),
			TotalSources:   totalSources,
			TestedNodes:    int(atomic.LoadInt64(&p.testedNodes)),
			TotalNodes:     int(atomic.LoadInt64(&p.totalNodes)),
			ActiveNodes:    int(atomic.LoadInt64(&p.activeNodes)),
			MaxNodes:       p.cfg.MaxNodes,
			BlacklistCount: p.bl.Count(),
			Duration:       time.Since(startTime),
		}
	})
	defer stopHeartbeat()

	initBlacklistCount := p.bl.Count()

	// 3. Step 1: Scrape & Decode
	logger.Info().Int("sources", totalSources).Msg("Phase 1: Gathering proxy nodes from subscription feeds")
	gatherRes, err := p.prov.GatherFromSources(ctx, sources)
	if err != nil {
		p.notif.SendError(err, "Subscription Gathering")
		return nil, fmt.Errorf("failed to gather subscriptions: %w", err)
	}

	totalScraped := gatherRes.TotalRawLines
	uniqueNodes := gatherRes.UniqueNodes
	atomic.StoreInt64(&p.scrapedSources, int64(totalSources))

	logger.Info().
		Int("total_scraped", totalScraped).
		Int("unique_nodes", len(uniqueNodes)).
		Msg("Phase 1 Complete: Subscriptions decoded and deduplicated")

	// 4. Step 2: Blacklist Pre-filtering
	var candidateNodes []*model.ProxyNode
	for _, n := range uniqueNodes {
		if !p.bl.ContainsNode(n) {
			candidateNodes = append(candidateNodes, n)
		}
	}

	deadPreFiltered := len(uniqueNodes) - len(candidateNodes)
	atomic.StoreInt64(&p.totalNodes, int64(len(candidateNodes)))

	logger.Info().
		Int("candidates", len(candidateNodes)).
		Int("filtered_dead", deadPreFiltered).
		Msg("Phase 2 Complete: Blacklisted nodes skipped")

	// 5. Step 3: Sandbox Verification
	logger.Info().
		Int("candidates", len(candidateNodes)).
		Int("concurrency", p.cfg.ConcurrencyWorkers).
		Msg("Phase 3: Initiating sandbox network verification")

	testResults := p.tester.TestAll(ctx, candidateNodes)

	var activeNodes []*model.ProxyNode
	topCountries := make(map[string]int)
	topProtocols := make(map[string]int)

	accountIdx := 1
	for _, res := range testResults {
		if len(res.TestedModes) == 0 || res.Node == nil {
			continue
		}

		// Separate account per verified connection mode (cdn, sni, direct)
		for _, mode := range res.TestedModes {
			mode = strings.ToLower(strings.TrimSpace(mode))
			if mode == "" {
				continue
			}

			cloned := *res.Node
			cloned.ConnMode = mode
			cloned.Remark = sandbox.FormatRemark(accountIdx, &cloned, cloned.CountryCode, cloned.Org, mode)
			if formatted, err := proxy.FormatString(&cloned); err == nil && formatted != "" {
				cloned.Raw = formatted
			}

			activeNodes = append(activeNodes, &cloned)
			accountIdx++

			if res.GeoIP.Country != "" {
				topCountries[res.GeoIP.Country]++
			}
			vpn := strings.ToLower(res.Node.VPN)
			if vpn != "" {
				topProtocols[vpn]++
			}

			if len(activeNodes) >= p.cfg.MaxNodes {
				break
			}
		}

		if len(activeNodes) >= p.cfg.MaxNodes {
			logger.Info().
				Int("limit", p.cfg.MaxNodes).
				Msg("Reached target max nodes threshold, stopping collection")
			break
		}
	}

	atomic.StoreInt64(&p.activeNodes, int64(len(activeNodes)))
	atomic.StoreInt64(&p.testedNodes, int64(len(candidateNodes)))

	deadDetected := (p.bl.Count() - initBlacklistCount)

	logger.Info().
		Int("active_verified", len(activeNodes)).
		Int("new_dead_detected", deadDetected).
		Msg("Phase 3 Complete: Sandbox testing finished")

	// 6. Step 4: Storage Persistence & Local Export
	var filesExported bool
	if len(activeNodes) > 0 {
		if exportErr := p.exporter.ExportAll(activeNodes); exportErr != nil {
			logger.Warn().Err(exportErr).Msg("Failed to export local subscription files")
			p.notif.SendError(exportErr, "File Export")
		} else {
			filesExported = true
		}
	}

	var databaseSynced bool
	if !p.cfg.DryRun && p.db != nil && len(activeNodes) > 0 {
		logger.Info().Msg("Phase 4: Committing verified proxies to Turso LibSQL database")
		dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
		defer dbCancel()

		if err := p.db.InitSchema(dbCtx); err != nil {
			logger.Error().Err(err).Msg("Failed to initialize database schema")
			p.notif.SendError(err, "Database Schema Init")
		} else {
			savedCount, saveErr := p.db.SaveBatch(dbCtx, activeNodes, true)
			if saveErr != nil {
				logger.Error().Err(saveErr).Msg("Failed to save proxy batch to database")
				p.notif.SendError(saveErr, "Database Batch Save")
			} else {
				databaseSynced = true
				logger.Info().Int("saved", savedCount).Msg("Persisted proxy batch to database successfully")
			}
		}
	} else if p.cfg.DryRun {
		logger.Info().Msg("Phase 4: Dry-run active; skipping remote database commit")
	}

	// 7. Step 5: Persist Blacklist
	if saveErr := p.bl.Save(p.cfg.BlacklistPath); saveErr != nil {
		logger.Error().Err(saveErr).Str("path", p.cfg.BlacklistPath).Msg("Failed to persist blacklist")
	} else {
		logger.Info().
			Int("total_blacklist", p.bl.Count()).
			Str("path", p.cfg.BlacklistPath).
			Msg("Blacklist saved successfully")
	}

	// 8. Step 6: Telemetry & Summary Report
	endTime := time.Now()
	summary := &notifier.RunSummary{
		StartTime:       startTime,
		EndTime:         endTime,
		Duration:        endTime.Sub(startTime),
		TotalSources:    totalSources,
		TotalScraped:    totalScraped,
		UniqueNodes:     len(uniqueNodes),
		BlacklistedInit: initBlacklistCount,
		BlacklistedEnd:  p.bl.Count(),
		DeadDetected:    deadDetected,
		TestedNodes:     len(candidateNodes),
		ActiveSaved:     len(activeNodes),
		DatabaseSynced:  databaseSynced,
		FilesExported:   filesExported,
		TopCountries:    topCountries,
		TopProtocols:    topProtocols,
	}

	p.notif.SendSummary(*summary)

	// Send diagnostic document report if accounts saved
	if len(activeNodes) > 0 {
		reportBytes := p.generateReport(summary, activeNodes)
		reportName := fmt.Sprintf("report_%s.txt", startTime.Format("20060102_150405"))
		_ = p.notif.SendDocument(reportName, reportBytes, "LatinaSub Execution Report")
	}

	logger.Info().
		Dur("duration", summary.Duration).
		Int("active_saved", summary.ActiveSaved).
		Msg("=== LatinaSub Pipeline Execution Finished Successfully ===")

	return summary, nil
}

// generateReport produces a concise human-readable text document of the run.
func (p *Pipeline) generateReport(summary *notifier.RunSummary, nodes []*model.ProxyNode) []byte {
	var sb strings.Builder
	sb.WriteString("=========================================\n")
	sb.WriteString("       LATINASUB EXECUTION REPORT        \n")
	sb.WriteString("=========================================\n\n")

	sb.WriteString(fmt.Sprintf("Start Time:    %s\n", summary.StartTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("End Time:      %s\n", summary.EndTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Duration:      %s\n", summary.Duration.Round(time.Second)))
	sb.WriteString(fmt.Sprintf("Total Scraped: %d\n", summary.TotalScraped))
	sb.WriteString(fmt.Sprintf("Unique Nodes:  %d\n", summary.UniqueNodes))
	sb.WriteString(fmt.Sprintf("Active Saved:  %d\n", summary.ActiveSaved))
	sb.WriteString(fmt.Sprintf("Dead Detected: %d\n", summary.DeadDetected))
	sb.WriteString(fmt.Sprintf("DB Synced:     %t\n", summary.DatabaseSynced))
	sb.WriteString(fmt.Sprintf("Files Export:  %t\n\n", summary.FilesExported))

	sb.WriteString("--- PROTOCOL BREAKDOWN ---\n")
	for proto, count := range summary.TopProtocols {
		sb.WriteString(fmt.Sprintf("  • %-12s: %d\n", proto, count))
	}

	sb.WriteString("\n--- TOP COUNTRIES ---\n")
	type countryCount struct {
		cc    string
		count int
	}
	var ccs []countryCount
	for cc, count := range summary.TopCountries {
		ccs = append(ccs, countryCount{cc: cc, count: count})
	}
	sort.Slice(ccs, func(i, j int) bool {
		return ccs[i].count > ccs[j].count
	})
	for _, item := range ccs {
		sb.WriteString(fmt.Sprintf("  • %-4s: %d\n", item.cc, item.count))
	}

	sb.WriteString("\n--- VERIFIED NODES SAMPLE (TOP 20) ---\n")
	limit := 20
	if len(nodes) < limit {
		limit = len(nodes)
	}
	for i := 0; i < limit; i++ {
		n := nodes[i]
		sb.WriteString(fmt.Sprintf("%2d. %-10s %-25s %s\n", i+1, strings.ToUpper(n.VPN), n.Server, n.Remark))
	}

	return []byte(sb.String())
}
