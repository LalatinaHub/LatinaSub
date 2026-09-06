package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/LalatinaHub/common/probe"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
)

// Runner manages the in-process execution of a sing-box instance and runs probe requests.
type Runner struct {
	clientTimeout time.Duration
}

// NewRunner creates a new Runner with configured request timeout.
func NewRunner(timeout time.Duration) *Runner {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Runner{
		clientTimeout: timeout,
	}
}

// Run executes the given sing-box configuration, initiates proxy verification to endpoints,
// and optionally performs extended YouTube CDN and Netflix unlocking probes.
func (r *Runner) Run(
	ctx context.Context,
	opt option.Options,
	listenPort int,
	endpoints []string,
	enableExtendedProbes bool,
) (GeoIPResult, int64, *probe.Result, *probe.Result, error) {
	var geo GeoIPResult

	boxCtx := newBoxContext(ctx)

	boxInstance, err := box.New(box.Options{
		Context: boxCtx,
		Options: opt,
	})
	if err != nil {
		return geo, 0, nil, nil, fmt.Errorf("failed to create sing-box instance: %w", err)
	}

	if err := boxInstance.Start(); err != nil {
		return geo, 0, nil, nil, fmt.Errorf("failed to start sing-box instance: %w", err)
	}
	defer boxInstance.Close()

	proxyURL, err := url.Parse(fmt.Sprintf("socks5://127.0.0.1:%d", listenPort))
	if err != nil {
		return geo, 0, nil, nil, fmt.Errorf("invalid proxy url: %w", err)
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: r.clientTimeout,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   r.clientTimeout,
	}

	var (
		lastErr   error
		latencyMs int64
		success   bool
	)

	if len(endpoints) == 0 {
		endpoints = []string{"https://myip.ipeek.workers.dev"}
	}

	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "curl/8.7.1")
		req.Header.Set("Accept", "*/*")

		start := time.Now()
		resp, err := httpClient.Do(req)
		latencyMs = time.Since(start).Milliseconds()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			decErr := json.NewDecoder(resp.Body).Decode(&geo)
			_ = resp.Body.Close()
			if decErr == nil && geo.Country != "" {
				geo.AsOrganization = SanitizeOrg(geo.AsOrganization)
				success = true
				break
			}
			lastErr = decErr
		} else {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("probe endpoint %s returned status %d", endpoint, resp.StatusCode)
		}
	}

	if !success {
		if lastErr == nil {
			lastErr = fmt.Errorf("no probe endpoints responded successfully")
		}
		return geo, latencyMs, nil, nil, lastErr
	}

	var (
		ytRes *probe.Result
		nfRes *probe.Result
	)

	if enableExtendedProbes {
		yt := probe.YouTubeCDN(ctx, httpClient)
		ytRes = &yt

		nf := probe.Netflix(ctx, httpClient)
		nfRes = &nf
	}

	return geo, latencyMs, ytRes, nfRes, nil
}
