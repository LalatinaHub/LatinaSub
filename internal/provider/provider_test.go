package provider

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSublistFromFile(t *testing.T) {
	p := New()

	// 1. Array of SubscriptionSource
	tempDir := t.TempDir()
	sourceJSON := `[
		{"id": 1, "remarks": "Test1", "url": "https://example.com/sub1", "enabled": true},
		{"id": 2, "remarks": "Test2", "url": "https://example.com/sub2", "enabled": false}
	]`
	file1 := filepath.Join(tempDir, "sources.json")
	require.NoError(t, os.WriteFile(file1, []byte(sourceJSON), 0644))

	sources, err := p.LoadSublistFromFile(file1)
	require.NoError(t, err)
	require.Len(t, sources, 2)
	assert.Equal(t, "Test1", sources[0].Remarks)
	assert.True(t, sources[0].Enabled)

	// 2. Array of string URLs
	urlJSON := `[
		"https://example.com/url1",
		"https://example.com/url2"
	]`
	file2 := filepath.Join(tempDir, "urls.json")
	require.NoError(t, os.WriteFile(file2, []byte(urlJSON), 0644))

	sources2, err := p.LoadSublistFromFile(file2)
	require.NoError(t, err)
	require.Len(t, sources2, 2)
	assert.Equal(t, "https://example.com/url1", sources2[0].URL)

	// 3. Invalid JSON
	file3 := filepath.Join(tempDir, "invalid.json")
	require.NoError(t, os.WriteFile(file3, []byte("invalid json"), 0644))
	_, err = p.LoadSublistFromFile(file3)
	assert.Error(t, err)
}

func TestGatherFromSources(t *testing.T) {
	trojan1 := "trojan://pass1@1.1.1.1:443?security=tls#Trojan1"
	vless1 := "vless://uuid1@2.2.2.2:443?type=ws&security=tls#Vless1"
	vmess1 := "vmess://eyJhZGQiOiIzLjMuMy4zIiwicG9ydCI6NDQzLCJpZCI6IjAwMDAwMDAwLTAwMDAtMDAwMC0wMDAwLTAwMDAwMDAwMDAwMCIsIm5ldCI6IndzIiwicHMiOiJWbWVzczEifQ=="

	// Server 1: Returns base64 encoded nodes
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := trojan1 + "\n" + vless1
		encoded := base64.StdEncoding.EncodeToString([]byte(payload))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(encoded))
	}))
	defer server1.Close()

	// Server 2: Returns raw lines with duplicate node
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := vmess1 + "\n" + trojan1 // trojan1 is duplicated
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer server2.Close()

	// Server 3: Returns 500 error
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server3.Close()

	p := New(
		WithWorkers(5),
		WithTimeout(3*time.Second),
	)

	sources := []SubscriptionSource{
		{ID: 1, Remarks: "Server 1", URL: server1.URL, Enabled: true},
		{ID: 2, Remarks: "Server 2 & 3", URL: server2.URL + "|" + server3.URL, Enabled: true},
		{ID: 3, Remarks: "Disabled", URL: "http://disabled.example.com", Enabled: false},
	}

	ctx := context.Background()
	result, err := p.GatherFromSources(ctx, sources)
	require.NoError(t, err)

	assert.Equal(t, 2, result.SuccessfulSources)
	assert.Equal(t, 1, result.FailedSources)
	assert.Equal(t, 4, result.TotalRawLines) // 2 from s1 + 2 from s2

	// Unique nodes should be 3 (trojan1, vless1, vmess1), since trojan1 was duplicated
	assert.Len(t, result.UniqueNodes, 3)

	vpns := make(map[string]bool)
	for _, n := range result.UniqueNodes {
		vpns[n.VPN] = true
	}
	assert.True(t, vpns["trojan"])
	assert.True(t, vpns["vless"])
	assert.True(t, vpns["vmess"])
}
