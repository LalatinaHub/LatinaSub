package sandbox

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LalatinaHub/LatinaSub/internal/blacklist"
	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/probe"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountryToEmoji(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"ID", "🇮🇩"},
		{"id", "🇮🇩"},
		{"SG", "🇸🇬"},
		{"US", "🇺🇸"},
		{"JP", "🇯🇵"},
		{"GB", "🇬🇧"},
		{"", "🌐"},
		{"XYZ", "🌐"},
		{"12", "🌐"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.expected, CountryToEmoji(tt.code))
		})
	}
}

func TestSanitizeOrg(t *testing.T) {
	assert.Equal(t, "Cloudflare Inc.", SanitizeOrg("Cloudflare, Inc."))
	assert.Equal(t, "PT. Telekomunikasi Selular", SanitizeOrg("PT. Telekomunikasi @Selular!!!"))
	assert.Equal(t, "DigitalOcean LLC", SanitizeOrg("  DigitalOcean   LLC  "))
}

func TestLookupRegion(t *testing.T) {
	reg := LookupRegion("CGK")
	assert.NotEmpty(t, reg)
	assert.Contains(t, reg, "JAKARTA")

	regSIN := LookupRegion("SIN")
	assert.NotEmpty(t, regSIN)
	assert.Contains(t, regSIN, "SINGAPORE")

	unknown := LookupRegion("ZZZ")
	assert.Equal(t, "ZZZ", unknown)
}

func TestFormatRemark(t *testing.T) {
	node := &model.ProxyNode{
		VPN:       "vmess",
		Transport: "ws",
		TLS:       true,
	}

	remark := FormatRemark(1, node, "ID", "Telkomsel", "cdn,sni")
	assert.Contains(t, remark, "[1]")
	assert.Contains(t, remark, "🇮🇩")
	assert.Contains(t, remark, "Telkomsel")
	assert.Contains(t, remark, "WS")
	assert.Contains(t, remark, "CDN,SNI")
	assert.Contains(t, remark, "TLS")
}

func TestEnrichNode(t *testing.T) {
	node := &model.ProxyNode{
		Server:    "example.com",
		VPN:       "vless",
		Transport: "grpc",
		TLS:       true,
	}

	geo := GeoIPResult{
		IP:             "103.10.2.1",
		Country:        "ID",
		AsOrganization: "Telkom Indonesia",
	}

	EnrichNode(5, node, geo, []string{"cdn"}, "CGK")

	assert.Equal(t, "103.10.2.1", node.IP)
	assert.Equal(t, "ID", node.CountryCode)
	assert.Equal(t, "Telkom Indonesia", node.Org)
	assert.Contains(t, node.Region, "JAKARTA")
	assert.Equal(t, "cdn", node.ConnMode)
	assert.Contains(t, node.Remark, "[5] 🇮🇩 Telkom Indonesia GRPC CDN TLS")
}

func TestBuildSingboxConfig(t *testing.T) {
	t.Run("VMess WS TLS in CDN Mode", func(t *testing.T) {
		node := &model.ProxyNode{
			Server:     "my-server.com",
			ServerPort: 443,
			VPN:        "vmess",
			UUID:       "00000000-0000-0000-0000-000000000000",
			Transport:  "ws",
			Host:       "original-host.com",
			Path:       "/vmess-ws",
			TLS:        true,
			SNI:        "sni.original.com",
		}

		opt, err := BuildSingboxConfig(node, 12345, ModeCDN, "104.18.2.2", "meet.google.com")
		require.NoError(t, err)
		assert.Len(t, opt.Inbounds, 1)
		assert.Len(t, opt.Outbounds, 4)

		mixedIn, ok := opt.Inbounds[0].Options.(*option.HTTPMixedInboundOptions)
		require.True(t, ok)
		assert.Equal(t, uint16(12345), mixedIn.ListenPort)

		vmessOb, ok := opt.Outbounds[0].Options.(*option.VMessOutboundOptions)
		require.True(t, ok)
		assert.Equal(t, "104.18.2.2", vmessOb.Server) // CDN mutation applied
		require.NotNil(t, vmessOb.TLS)
		assert.True(t, vmessOb.TLS.Enabled)
	})

	t.Run("VLESS GRPC TLS in SNI Mode", func(t *testing.T) {
		node := &model.ProxyNode{
			Server:      "my-vless.com",
			ServerPort:  443,
			VPN:         "vless",
			UUID:        "11111111-1111-1111-1111-111111111111",
			Transport:   "grpc",
			ServiceName: "vless-service",
			TLS:         true,
			SNI:         "old-sni.com",
		}

		opt, err := BuildSingboxConfig(node, 12346, ModeSNI, "104.18.2.2", "meet.google.com")
		require.NoError(t, err)
		assert.Len(t, opt.Outbounds, 4)

		vlessOb, ok := opt.Outbounds[0].Options.(*option.VLESSOutboundOptions)
		require.True(t, ok)
		assert.Equal(t, "my-vless.com", vlessOb.Server)
		require.NotNil(t, vlessOb.TLS)
		assert.Equal(t, "meet.google.com", vlessOb.TLS.ServerName) // SNI mutation applied
		assert.True(t, vlessOb.TLS.Insecure)
	})

	t.Run("Trojan WS in SNI Mode", func(t *testing.T) {
		node := &model.ProxyNode{
			Server:     "my-trojan.com",
			ServerPort: 443,
			VPN:        "trojan",
			Password:   "secret-pass",
			Transport:  "ws",
			Host:       "old-host.com",
			TLS:        true,
		}

		opt, err := BuildSingboxConfig(node, 12347, ModeSNI, "104.18.2.2", "meet.google.com")
		require.NoError(t, err)

		trojanOb, ok := opt.Outbounds[0].Options.(*option.TrojanOutboundOptions)
		require.True(t, ok)
		assert.Equal(t, "secret-pass", trojanOb.Password)
		require.NotNil(t, trojanOb.TLS)
		assert.Equal(t, "meet.google.com", trojanOb.TLS.ServerName)
	})

	t.Run("Shadowsocks Node", func(t *testing.T) {
		node := &model.ProxyNode{
			Server:     "198.51.100.1",
			ServerPort: 8388,
			VPN:        "shadowsocks",
			Method:     "aes-128-gcm",
			Password:   "ss-password",
		}

		opt, err := BuildSingboxConfig(node, 12348, ModeDirect, "", "")
		require.NoError(t, err)

		ssOb, ok := opt.Outbounds[0].Options.(*option.ShadowsocksOutboundOptions)
		require.True(t, ok)
		assert.Equal(t, "aes-128-gcm", ssOb.Method)
		assert.Equal(t, "ss-password", ssOb.Password)
	})

	t.Run("Nil Node Returns Error", func(t *testing.T) {
		_, err := BuildSingboxConfig(nil, 12349, ModeDirect, "", "")
		assert.Error(t, err)
	})
}

// MockRunner implements BoxRunner for unit testing without live network traffic.
type MockRunner struct {
	ShouldFail bool
	ShouldPanic bool
	Geo        GeoIPResult
	Latency    int64
	CallCount  int64
}

func (m *MockRunner) Run(
	ctx context.Context,
	opt option.Options,
	listenPort int,
	endpoints []string,
	enableExtendedProbes bool,
) (GeoIPResult, int64, *probe.Result, *probe.Result, error) {
	atomic.AddInt64(&m.CallCount, 1)

	if m.ShouldPanic {
		panic("mock runner simulated panic")
	}

	if m.ShouldFail {
		return GeoIPResult{}, 0, nil, nil, errors.New("connection timed out")
	}

	var yt *probe.Result
	if enableExtendedProbes {
		yt = &probe.Result{
			Name:     "YouTube CDN",
			Passed:   true,
			IATACode: "CGK",
			Region:   "JAKARTA",
		}
	}

	return m.Geo, m.Latency, yt, nil, nil
}

func TestTesterWithMockRunner(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Concurrency = 4
	bl := blacklist.New()

	nodeSuccess := &model.ProxyNode{
		Server:     "good.server.com",
		ServerPort: 443,
		VPN:        "vmess",
		UUID:       "00000000-0000-0000-0000-000000000001",
		TLS:        true,
	}

	nodeFail := &model.ProxyNode{
		Server:     "dead.server.com",
		ServerPort: 443,
		VPN:        "vmess",
		UUID:       "00000000-0000-0000-0000-000000000002",
		TLS:        true,
	}

	t.Run("Successful node testing and enrichment", func(t *testing.T) {
		tester := NewTester(cfg, bl)
		mock := &MockRunner{
			Geo: GeoIPResult{
				IP:             "103.10.10.1",
				Country:        "SG",
				AsOrganization: "Cloudflare, Inc.",
			},
			Latency: 45,
		}
		tester.SetRunner(mock)

		res, err := tester.TestNode(context.Background(), nodeSuccess, 1)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, "SG", res.GeoIP.Country)
		assert.Equal(t, "Cloudflare Inc.", res.GeoIP.AsOrganization)
		assert.Contains(t, nodeSuccess.Remark, "🇸🇬")
		assert.Contains(t, nodeSuccess.Remark, "Cloudflare Inc.")
		assert.False(t, bl.ContainsNode(nodeSuccess))
	})

	t.Run("Failing node added to blacklist", func(t *testing.T) {
		tester := NewTester(cfg, bl)
		mock := &MockRunner{
			ShouldFail: true,
		}
		tester.SetRunner(mock)

		res, err := tester.TestNode(context.Background(), nodeFail, 2)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.True(t, bl.ContainsNode(nodeFail), "dead node should be added to blacklist")
	})

	t.Run("Blacklisted node is skipped", func(t *testing.T) {
		tester := NewTester(cfg, bl)
		mock := &MockRunner{}
		tester.SetRunner(mock)

		// nodeFail is already in blacklist from previous subtest
		res, err := tester.TestNode(context.Background(), nodeFail, 3)
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "blacklisted")
		assert.Equal(t, int64(0), mock.CallCount, "mock runner should not be called for blacklisted node")
	})

	t.Run("Panic recovery per worker", func(t *testing.T) {
		tester := NewTester(cfg, bl)
		mock := &MockRunner{
			ShouldPanic: true,
		}
		tester.SetRunner(mock)

		panickingNode := &model.ProxyNode{
			Server:     "panic.server.com",
			ServerPort: 443,
			VPN:        "trojan",
			Password:   "panic-pass",
			TLS:        true,
		}

		// TestAll must not crash even if a worker panics
		results := tester.TestAll(context.Background(), []*model.ProxyNode{panickingNode})
		assert.Empty(t, results)
	})

	t.Run("Concurrent TestAll execution", func(t *testing.T) {
		blFresh := blacklist.New()
		tester := NewTester(cfg, blFresh)
		tester.cfg.EnableExtendedProbes = true
		mock := &MockRunner{
			Geo: GeoIPResult{
				IP:             "103.20.20.20",
				Country:        "ID",
				AsOrganization: "Biznet Networks",
			},
			Latency: 20,
		}
		tester.SetRunner(mock)

		nodes := make([]*model.ProxyNode, 10)
		for i := 0; i < 10; i++ {
			nodes[i] = &model.ProxyNode{
				Server:     "batch.server.com",
				ServerPort: 443,
				VPN:        "vmess",
				UUID:       "00000000-0000-0000-0000-00000000000" + string(rune('a'+i)),
				TLS:        true,
			}
		}

		results := tester.TestAll(context.Background(), nodes)
		assert.Len(t, results, 10)
		for _, res := range results {
			assert.Equal(t, "ID", res.GeoIP.Country)
			assert.Equal(t, "Biznet Networks", res.GeoIP.AsOrganization)
			assert.NotNil(t, res.YouTubeCDN)
			assert.Equal(t, "CGK", res.YouTubeCDN.IATACode)
		}
	})
}

func TestBuildSingboxConfig_RealNodeExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network test in short mode")
	}

	node := &model.ProxyNode{
		VPN: "direct",
	}

	opt, err := BuildSingboxConfig(node, 10899, ModeDirect, "", "")
	require.NoError(t, err)

	runner := NewRunner(10 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	geo, lat, _, _, err := runner.Run(ctx, opt, 10899, []string{"https://myip.ipeek.workers.dev"}, false)
	t.Logf("Result: err=%v, lat=%d ms, geo=%+v", err, lat, geo)
	assert.NoError(t, err)
	assert.NotEmpty(t, geo.Country)
}

