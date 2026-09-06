package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
)

// DefaultUserAgent used to fetch subscription feeds safely.
const DefaultUserAgent = "v2rayNG/1.8.5 (Linux; Android 12) Clash.Meta/1.14.0"

// Provider orchestrates gathering and fetching of proxy subscriptions.
type Provider struct {
	client     *http.Client
	workers    int
	maxTimeout time.Duration
	userAgent  string
}

// ProviderOption defines a configuration functional option for Provider.
type ProviderOption func(*Provider)

// WithWorkers sets the maximum concurrency worker limit.
func WithWorkers(workers int) ProviderOption {
	return func(p *Provider) {
		if workers > 0 {
			p.workers = workers
		}
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(timeout time.Duration) ProviderOption {
	return func(p *Provider) {
		if timeout > 0 {
			p.maxTimeout = timeout
		}
	}
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) ProviderOption {
	return func(p *Provider) {
		if ua != "" {
			p.userAgent = ua
		}
	}
}

// New creates a new Provider instance with optimal connection pooling.
func New(opts ...ProviderOption) *Provider {
	p := &Provider{
		workers:    20,
		maxTimeout: 10 * time.Second,
		userAgent:  DefaultUserAgent,
	}

	for _, opt := range opts {
		opt(p)
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DisableCompression: false,
	}

	p.client = &http.Client{
		Transport: transport,
		Timeout:   p.maxTimeout,
	}

	return p
}

// LoadSublistFromFile loads subscription feeds from a local JSON file.
// The file may contain either []SubscriptionSource or []string (raw URLs).
// If any URLs point to remote sublists (e.g. ending in .json), they are fetched and expanded recursively.
func (p *Provider) LoadSublistFromFile(filePath string) ([]SubscriptionSource, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sublist file %s: %w", filePath, err)
	}

	sources, err := p.parseSublistJSON(data)
	if err != nil {
		return nil, err
	}

	return p.ExpandSources(context.Background(), sources)
}

// ExpandSources inspects subscription sources and recursively expands any sources
// whose URLs point to remote JSON sublists (such as Mineral/sub.json, etc.).
func (p *Provider) ExpandSources(ctx context.Context, sources []SubscriptionSource) ([]SubscriptionSource, error) {
	var (
		expanded []SubscriptionSource
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	for _, s := range sources {
		trimmedURL := strings.TrimSpace(s.URL)
		if trimmedURL == "" {
			continue
		}

		if strings.HasSuffix(strings.ToLower(trimmedURL), ".json") {
			wg.Add(1)
			go func(src SubscriptionSource) {
				defer wg.Done()
				remoteSubs, err := p.FetchRemoteSublist(ctx, src.URL)
				if err != nil || len(remoteSubs) == 0 {
					logger.Warn().Err(err).Str("url", src.URL).Msg("Could not expand remote sublist; using as direct source")
					mu.Lock()
					expanded = append(expanded, src)
					mu.Unlock()
					return
				}

				mu.Lock()
				expanded = append(expanded, remoteSubs...)
				mu.Unlock()
			}(s)
		} else {
			expanded = append(expanded, s)
		}
	}

	wg.Wait()
	return expanded, nil
}

// FetchRemoteSublist fetches a remote sublist URL and parses it.
func (p *Provider) FetchRemoteSublist(ctx context.Context, url string) ([]SubscriptionSource, error) {
	body, err := p.fetchHTTP(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote sublist from %s: %w", url, err)
	}

	return p.parseSublistJSON([]byte(body))
}

func (p *Provider) parseSublistJSON(data []byte) ([]SubscriptionSource, error) {
	// 1. Try parsing as []SubscriptionSource
	var sources []SubscriptionSource
	if err := json.Unmarshal(data, &sources); err == nil && len(sources) > 0 {
		hasValidURL := false
		for _, s := range sources {
			if strings.TrimSpace(s.URL) != "" {
				hasValidURL = true
				break
			}
		}
		if hasValidURL {
			return sources, nil
		}
	}

	// 2. Try parsing as []string (list of URLs)
	var urls []string
	if err := json.Unmarshal(data, &urls); err == nil && len(urls) > 0 {
		var result []SubscriptionSource
		for i, u := range urls {
			trimmed := strings.TrimSpace(u)
			if trimmed != "" {
				result = append(result, SubscriptionSource{
					ID:      i + 1,
					Remarks: fmt.Sprintf("Source-%d", i+1),
					URL:     trimmed,
					Enabled: true,
				})
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	return nil, fmt.Errorf("unrecognized sublist JSON schema")
}

// GatherFromSources fetches subscriptions concurrently from a list of sources,
// decodes, normalizes, parses, and returns unique *model.ProxyNode instances.
func (p *Provider) GatherFromSources(ctx context.Context, sources []SubscriptionSource) (*GatherResult, error) {
	var (
		wg      sync.WaitGroup
		sem     = make(chan struct{}, p.workers)
		mu      sync.Mutex
		allURLs []string
		stats   = &GatherResult{
			TotalSources: len(sources),
		}
	)

	for srcIdx, src := range sources {
		if !src.Enabled || strings.TrimSpace(src.URL) == "" {
			continue
		}

		// Support multi-URL separated by pipe in single source
		subURLs := strings.Split(src.URL, "|")
		totalSubs := len(subURLs)

		for subIdx, subURL := range subURLs {
			subURL = strings.TrimSpace(subURL)
			if subURL == "" {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(sIdx, subI, totSubs int, targetURL string) {
				defer func() {
					if r := recover(); r != nil {
						logger.Error().Msgf("Recovered from panic while fetching %s: %v", targetURL, r)
					}
					<-sem
					wg.Done()
				}()

				body, err := p.fetchHTTP(ctx, targetURL)
				if err != nil {
					logger.Debug().Err(err).Str("url", targetURL).Msg("Failed to fetch subscription feed")
					mu.Lock()
					stats.FailedSources++
					mu.Unlock()
					return
				}

				extracted := ExtractProxyURLs(body)
				if len(extracted) == 0 {
					// Fallback: check if body is a nested JSON array of sub-sources
					var nested []SubscriptionSource
					if err := json.Unmarshal([]byte(body), &nested); err == nil && len(nested) > 0 {
						for _, ns := range nested {
							if !ns.Enabled || strings.TrimSpace(ns.URL) == "" {
								continue
							}
							for _, nestedURL := range strings.Split(ns.URL, "|") {
								nestedURL = strings.TrimSpace(nestedURL)
								if nestedURL == "" {
									continue
								}
								nestedBody, err := p.fetchHTTP(ctx, nestedURL)
								if err != nil {
									continue
								}
								nestedExtracted := ExtractProxyURLs(nestedBody)
								if len(nestedExtracted) > 0 {
									mu.Lock()
									stats.SuccessfulSources++
									stats.TotalRawLines += len(nestedExtracted)
									allURLs = append(allURLs, nestedExtracted...)
									currTotal := stats.TotalRawLines
									mu.Unlock()

									logger.Info().
										Msgf("[[%d/%d]%d/%d] [%d] [%d] %s", subI, totSubs, sIdx, len(sources), len(nestedExtracted), currTotal, nestedURL)
								}
							}
						}
					}
					return
				}

				mu.Lock()
				stats.SuccessfulSources++
				stats.TotalRawLines += len(extracted)
				allURLs = append(allURLs, extracted...)
				currTotal := stats.TotalRawLines
				mu.Unlock()

				logger.Info().
					Msgf("[[%d/%d]%d/%d] [%d] [%d] %s", subI, totSubs, sIdx, len(sources), len(extracted), currTotal, targetURL)
			}(srcIdx, subIdx, totalSubs, subURL)
		}
	}

	wg.Wait()

	// Parse valid proxy nodes using common/proxy.Parser
	nodes := ParseNodes(allURLs)
	uniqueNodes := DeduplicateNodes(nodes)
	stats.UniqueNodes = uniqueNodes

	logger.Info().
		Int("total_raw", stats.TotalRawLines).
		Int("parsed_nodes", len(nodes)).
		Int("unique_nodes", len(uniqueNodes)).
		Msg("Subscription gathering completed")

	return stats, nil
}

func (p *Provider) fetchHTTP(ctx context.Context, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Read with limit (up to 15MB per subscription feed to prevent memory exhaustion)
	limitedReader := io.LimitReader(resp.Body, 15*1024*1024)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
