package provider

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/LalatinaHub/common/model"
	"github.com/LalatinaHub/common/proxy"
)

// DecodeBase64Safe safely decodes a base64 string, automatically handling standard,
// URL-safe encodings, missing padding, and mixed formats.
// If the content is already plaintext proxy links or decoding fails, original is returned.
func DecodeBase64Safe(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	// If it already has multiple protocol prefixes, it's likely plaintext
	if hasMultipleProxyPrefixes(trimmed) {
		return trimmed
	}

	clean := strings.ReplaceAll(trimmed, "\r", "")
	clean = strings.ReplaceAll(clean, "\n", "")
	clean = strings.ReplaceAll(clean, " ", "")

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.RawURLEncoding,
	}

	for _, enc := range encodings {
		if decoded, err := enc.DecodeString(clean); err == nil {
			str := string(decoded)
			if strings.Contains(str, "://") {
				return str
			}
		}
	}

	// Try fixing padding if length is not divisible by 4
	if rem := len(clean) % 4; rem != 0 {
		padded := clean + strings.Repeat("=", 4-rem)
		for _, enc := range encodings {
			if decoded, err := enc.DecodeString(padded); err == nil {
				str := string(decoded)
				if strings.Contains(str, "://") {
					return str
				}
			}
		}
	}

	return trimmed
}

func hasMultipleProxyPrefixes(text string) bool {
	count := 0
	for _, proto := range AcceptedProtocols {
		count += strings.Count(text, proto)
		if count > 1 {
			return true
		}
	}
	return false
}

// SplitLines splits text across common separator tokens (\r\n, \n, |, ,, <br/>, <br>).
func SplitLines(content string) []string {
	normalized := content
	for _, sep := range ConfigSeparators {
		normalized = strings.ReplaceAll(normalized, sep, "\n")
	}

	rawParts := strings.Split(normalized, "\n")
	var result []string
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") {
			result = append(result, trimmed)
		}
	}
	return result
}

// ExtractProxyURLs decodes and parses a subscription response body into individual proxy URLs.
func ExtractProxyURLs(body string) []string {
	decoded := DecodeBase64Safe(body)
	lines := SplitLines(decoded)

	var proxies []string
	for _, line := range lines {
		// Clean leading/trailing junk
		line = strings.TrimSpace(line)
		for _, proto := range AcceptedProtocols {
			if idx := strings.Index(line, proto); idx != -1 {
				cleanProxy := strings.Trim(line[idx:], " \"'`,<>()[]\r\n\t")
				if cleanProxy != "" {
					proxies = append(proxies, cleanProxy)
					break
				}
			}
		}
	}
	return proxies
}

// ParseNodes parses a slice of raw proxy URL strings into valid *model.ProxyNode instances
// using the pure-Go LalatinaHub/common/proxy.Parser.
func ParseNodes(rawURLs []string) []*model.ProxyNode {
	parser := proxy.NewParser()
	var nodes []*model.ProxyNode

	for _, raw := range rawURLs {
		node, err := parser.Parse(raw)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// NodeFingerprint creates a deterministic unique identifier for a proxy node.
func NodeFingerprint(node *model.ProxyNode) string {
	if node == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s",
		strings.ToLower(node.VPN),
		strings.ToLower(node.Server),
		node.ServerPort,
		node.UUID,
		node.Password,
		strings.ToLower(node.Transport),
		node.Path,
		strings.ToLower(node.SNI),
	)
}

// DeduplicateNodes removes duplicate ProxyNodes based on their configuration fingerprint.
func DeduplicateNodes(nodes []*model.ProxyNode) []*model.ProxyNode {
	seen := make(map[string]struct{}, len(nodes))
	var unique []*model.ProxyNode

	for _, node := range nodes {
		fp := NodeFingerprint(node)
		if fp == "" {
			continue
		}
		if _, exists := seen[fp]; !exists {
			seen[fp] = struct{}{}
			unique = append(unique, node)
		}
	}
	return unique
}
