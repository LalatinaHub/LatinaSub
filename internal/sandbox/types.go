package sandbox

import (
	"time"

	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/probe"
)

// Mode represents the connectivity test mutation mode.
type Mode string

const (
	ModeCDN    Mode = "cdn"
	ModeSNI    Mode = "sni"
	ModeDirect Mode = "direct"
)

// DefaultTestModes contains the standard modes to evaluate.
var DefaultTestModes = []Mode{ModeCDN, ModeSNI, ModeDirect}

// GeoIPResult holds the egress network telemetry from probe endpoints.
type GeoIPResult struct {
	IP             string `json:"ip"`
	Proxy          string `json:"proxy,omitempty"`
	Port           int64  `json:"port,omitempty"`
	Country        string `json:"country"`
	AsOrganization string `json:"asOrganization"`
}

// TestResult stores the outcome of proxy node evaluation.
type TestResult struct {
	Node        *model.ProxyNode
	TestedModes []string
	GeoIP       GeoIPResult
	RawURI      string
	LatencyMs   int64
	YouTubeCDN  *probe.Result
	Netflix     *probe.Result
}

// Config provides configuration options for the sandbox environment.
type Config struct {
	Timeout              time.Duration
	CDNHost              string
	SNIHost              string
	Endpoints            []string
	Concurrency          int
	EnableExtendedProbes bool
	MaxNodes             int
}

// DefaultConfig provides sensible defaults matching the megalodon and LatinaHub specification.
func DefaultConfig() Config {
	return Config{
		Timeout:              5 * time.Second,
		CDNHost:              "104.18.2.2",
		SNIHost:              "meet.google.com",
		Endpoints:            []string{"https://myip.ipeek.workers.dev"},
		Concurrency:          50,
		EnableExtendedProbes: false,
	}
}
