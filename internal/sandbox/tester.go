package sandbox

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/LalatinaHub/LatinaSub/internal/blacklist"
	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/LatinaSub/pkg/netutil"
	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/probe"
	"github.com/sagernet/sing-box/option"
)

// BoxRunner defines the execution contract for in-process proxy testing.
type BoxRunner interface {
	Run(ctx context.Context, opt option.Options, listenPort int, endpoints []string, enableExtendedProbes bool) (GeoIPResult, int64, *probe.Result, *probe.Result, error)
}

// Tester manages concurrent proxy verification, mode mutations, blacklisting, and enrichment.
type Tester struct {
	cfg       Config
	bl        *blacklist.Blacklist
	runner    BoxRunner
	testModes []Mode

	mu      sync.Mutex
	results []*TestResult
}

// NewTester creates a new Tester instance.
func NewTester(cfg Config, bl *blacklist.Blacklist) *Tester {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 50
	}
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"https://myip.ipeek.workers.dev"}
	}

	return &Tester{
		cfg:       cfg,
		bl:        bl,
		runner:    NewRunner(cfg.Timeout),
		testModes: DefaultTestModes,
		results:   make([]*TestResult, 0),
	}
}

// SetRunner allows injecting a custom or mock BoxRunner (useful for testing).
func (t *Tester) SetRunner(runner BoxRunner) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runner = runner
}

// SetModes configures the test modes to evaluate (e.g. cdn, sni, direct).
func (t *Tester) SetModes(modes []Mode) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.testModes = modes
}

// SetMaxNodes sets the maximum number of passing nodes before stopping test dispatch.
func (t *Tester) SetMaxNodes(max int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cfg.MaxNodes = max
}

// Results returns a copy of verified proxy test results.
func (t *Tester) Results() []*TestResult {
	t.mu.Lock()
	defer t.mu.Unlock()
	res := make([]*TestResult, len(t.results))
	copy(res, t.results)
	return res
}

// Reset clears existing test results.
func (t *Tester) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.results = t.results[:0]
}

// TestNode evaluates a single proxy node across configured mutation modes.
func (t *Tester) TestNode(ctx context.Context, node *model.ProxyNode, index int) (*TestResult, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}

	// 1. Blacklist check
	if t.bl != nil && t.bl.ContainsNode(node) {
		return nil, fmt.Errorf("node is blacklisted: %s", blacklist.HashNode(node))
	}

	var (
		passedModes []string
		lastGeo     GeoIPResult
		bestLatency int64 = -1
		ytRes       *probe.Result
		nfRes       *probe.Result
	)

	// 2. Test configured modes (CDN, SNI, etc.)
	for _, mode := range t.testModes {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		port, releasePort, err := netutil.AllocatePort()
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to allocate ephemeral port for sandbox test")
			continue
		}

		opt, err := BuildSingboxConfig(node, port, mode, t.cfg.CDNHost, t.cfg.SNIHost)
		if err != nil {
			releasePort()
			continue
		}

		testCtx, cancel := context.WithTimeout(ctx, t.cfg.Timeout)
		geo, latency, ytR, nfR, testErr := t.runner.Run(testCtx, opt, port, t.cfg.Endpoints, t.cfg.EnableExtendedProbes)
		cancel()
		releasePort()

		if testErr == nil && geo.Country != "" {
			passedModes = append(passedModes, string(mode))
			lastGeo = geo
			if bestLatency == -1 || latency < bestLatency {
				bestLatency = latency
			}
			if ytR != nil && ytR.Passed {
				ytRes = ytR
			}
			if nfR != nil && nfR.Passed {
				nfRes = nfR
			}
		}
	}

	// 3. Post-evaluation
	if len(passedModes) == 0 {
		// All modes failed: mark as dead account in blacklist
		if t.bl != nil {
			t.bl.AddNode(node)
		}
		return nil, fmt.Errorf("node failed all connection modes")
	}

	// 4. Enrich node metadata and remark
	var iataCode string
	if ytRes != nil && ytRes.IATACode != "" {
		iataCode = ytRes.IATACode
	}
	EnrichNode(index, node, lastGeo, passedModes, iataCode)

	lastGeo.AsOrganization = SanitizeOrg(lastGeo.AsOrganization)
	result := &TestResult{
		Node:        node,
		TestedModes: passedModes,
		GeoIP:       lastGeo,
		LatencyMs:   bestLatency,
		YouTubeCDN:  ytRes,
		Netflix:     nfRes,
	}

	t.mu.Lock()
	t.results = append(t.results, result)
	t.mu.Unlock()

	return result, nil
}

// TestAll runs concurrent evaluations over a collection of proxy nodes.
func (t *Tester) TestAll(ctx context.Context, nodes []*model.ProxyNode) []*TestResult {
	total := len(nodes)
	if total == 0 {
		return nil
	}

	concurrency := t.cfg.Concurrency
	if concurrency > total {
		concurrency = total
	}

	sem := make(chan struct{}, concurrency)
	var (
		wg        sync.WaitGroup
		completed int64
		passed    int64
	)

	logger.Info().
		Int("total_nodes", total).
		Int("concurrency", concurrency).
		Msg("Starting sandbox verification worker pool")

	for i, node := range nodes {
		if t.cfg.MaxNodes > 0 && atomic.LoadInt64(&passed) >= int64(t.cfg.MaxNodes) {
			logger.Info().
				Int64("passed", atomic.LoadInt64(&passed)).
				Int("max_nodes", t.cfg.MaxNodes).
				Msg("Target max verified nodes reached; stopping test dispatch")
			break
		}

		select {
		case <-ctx.Done():
			logger.Warn().Msg("Sandbox test run cancelled by context")
			break
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int, target *model.ProxyNode) {
			defer wg.Done()
			defer func() { <-sem }()

			// Panic recovery per goroutine worker
			defer func() {
				if r := recover(); r != nil {
					logger.Error().
						Interface("panic", r).
						Str("server", target.Server).
						Msg("Recovered from panic in sandbox worker goroutine")
				}
			}()

			res, err := t.TestNode(ctx, target, idx+1)
			done := atomic.AddInt64(&completed, 1)

			if err == nil && res != nil {
				succ := atomic.AddInt64(&passed, 1)
				logger.Info().
					Int64("completed", done).
					Int("total", total).
					Int64("passed", succ).
					Str("country", res.GeoIP.Country).
					Str("org", res.GeoIP.AsOrganization).
					Strs("modes", res.TestedModes).
					Str("server", target.Server).
					Msg("Proxy node passed verification")
			} else {
				logger.Debug().
					Int64("completed", done).
					Int("total", total).
					Str("server", target.Server).
					Err(err).
					Msg("Proxy node failed verification")
			}
		}(i, node)
	}

	wg.Wait()

	logger.Info().
		Int("total", total).
		Int64("passed", atomic.LoadInt64(&passed)).
		Msg("Sandbox verification finished")

	return t.Results()
}
