package provider

import (
	"github.com/LalatinaHub/common/model"
)

// AcceptedProtocols are the proxy protocols accepted by LatinaSub.
var AcceptedProtocols = []string{
	"vmess://",
	"vless://",
	"trojan://",
	"ss://",
}

// ConfigSeparators are delimiters used by various subscription aggregators.
var ConfigSeparators = []string{
	"\r\n",
	"\n",
	"|",
	",",
	"<br/>",
	"<br>",
}

// SubscriptionSource represents a subscription feed metadata entry.
type SubscriptionSource struct {
	ID           int    `json:"id,omitempty"`
	Remarks      string `json:"remarks,omitempty"`
	Site         string `json:"site,omitempty"`
	URL          string `json:"url"`
	UpdateMethod string `json:"update_method,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// GatherResult contains the outcome of a scraping run.
type GatherResult struct {
	TotalSources      int
	SuccessfulSources int
	FailedSources     int
	TotalRawLines     int
	UniqueNodes       []*model.ProxyNode
}
