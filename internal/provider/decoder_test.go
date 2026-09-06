package provider

import (
	"encoding/base64"
	"testing"

	"github.com/LalatinaHub/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBase64Safe(t *testing.T) {
	rawProxy := "vmess://eyJhZGQiOiIxLjEuMS4xIiwicG9ydCI6NDQzLCJpZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMCIsIm5ldCI6IndzIiwicHMiOiJ0ZXN0In0="

	// 1. Plaintext input
	assert.Equal(t, rawProxy, DecodeBase64Safe(rawProxy))

	// 2. Standard Base64
	stdEncoded := base64.StdEncoding.EncodeToString([]byte(rawProxy))
	assert.Equal(t, rawProxy, DecodeBase64Safe(stdEncoded))

	// 3. Raw (unpadded) Base64
	rawEncoded := base64.RawStdEncoding.EncodeToString([]byte(rawProxy))
	assert.Equal(t, rawProxy, DecodeBase64Safe(rawEncoded))

	// 4. URL-Safe Base64
	urlEncoded := base64.URLEncoding.EncodeToString([]byte(rawProxy))
	assert.Equal(t, rawProxy, DecodeBase64Safe(urlEncoded))

	// 5. Empty string
	assert.Equal(t, "", DecodeBase64Safe(""))
}

func TestSplitLines(t *testing.T) {
	content := "line1\r\nline2|line3,line4<br/>line5<br>line6\n#comment\n//comment2\nline7"
	lines := SplitLines(content)

	assert.Contains(t, lines, "line1")
	assert.Contains(t, lines, "line2")
	assert.Contains(t, lines, "line3")
	assert.Contains(t, lines, "line4")
	assert.Contains(t, lines, "line5")
	assert.Contains(t, lines, "line6")
	assert.Contains(t, lines, "line7")
	assert.NotContains(t, lines, "#comment")
	assert.NotContains(t, lines, "//comment2")
}

func TestExtractProxyURLs(t *testing.T) {
	trojanURL := "trojan://pass@1.2.3.4:443?security=tls#TrojanNode"
	vlessURL := "vless://uuid@5.6.7.8:443?type=ws&security=tls#VlessNode"
	unsupported := "http://example.com/not-a-proxy"

	payload := trojanURL + "\n" + unsupported + "\n" + vlessURL
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))

	proxies := ExtractProxyURLs(encoded)
	require.Len(t, proxies, 2)
	assert.Equal(t, trojanURL, proxies[0])
	assert.Equal(t, vlessURL, proxies[1])
}

func TestParseAndDeduplicateNodes(t *testing.T) {
	trojanURL := "trojan://pass@1.2.3.4:443?security=tls#TrojanNode"
	vlessURL := "vless://uuid@5.6.7.8:443?type=ws&security=tls#VlessNode"

	urls := []string{trojanURL, vlessURL, trojanURL} // has duplicate
	nodes := ParseNodes(urls)
	require.Len(t, nodes, 3)

	unique := DeduplicateNodes(nodes)
	require.Len(t, unique, 2)

	assert.Equal(t, "trojan", unique[0].VPN)
	assert.Equal(t, "1.2.3.4", unique[0].Server)
	assert.Equal(t, "vless", unique[1].VPN)
	assert.Equal(t, "5.6.7.8", unique[1].Server)
}

func TestNodeFingerprint(t *testing.T) {
	assert.Empty(t, NodeFingerprint(nil))

	node := &model.ProxyNode{
		VPN:        "trojan",
		Server:     "example.com",
		ServerPort: 443,
		Password:   "secret",
	}
	fp1 := NodeFingerprint(node)
	assert.NotEmpty(t, fp1)

	node2 := &model.ProxyNode{
		VPN:        "TROJAN",
		Server:     "EXAMPLE.COM",
		ServerPort: 443,
		Password:   "secret",
		Remark:     "Different Remark",
	}
	fp2 := NodeFingerprint(node2)
	assert.Equal(t, fp1, fp2, "Fingerprint should ignore remark and be case-insensitive on vpn/server")
}
