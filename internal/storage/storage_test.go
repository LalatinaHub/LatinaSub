package storage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LalatinaHub/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDatabase_InitSchema(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	storageDB, err := NewDatabase(mockDB)
	require.NoError(t, err)

	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta(createTableQuery)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(createIndexQuery)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(createIndexCC)).WillReturnResult(sqlmock.NewResult(0, 0))

	err = storageDB.InitSchema(ctx)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDatabase_SaveBatch(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	storageDB, err := NewDatabase(mockDB)
	require.NoError(t, err)

	ctx := context.Background()

	nodes := []*model.ProxyNode{
		{
			Server:      "id-server.net",
			IP:          "103.1.2.3",
			ServerPort:  443,
			UUID:        "00000000-0000-0000-0000-000000000001",
			Security:    "auto",
			Host:        "id-server.net",
			TLS:         true,
			Transport:   "ws",
			Path:        "/vmess'ws", // contains single quote to verify escaping
			Remark:      "[1] 🇮🇩 Dicky's Fast Server WS CDN TLS",
			ConnMode:    "cdn,sni",
			CountryCode: "ID",
			Region:      "JAKARTA",
			Org:         "PT 'Telkom' Indonesia",
			VPN:         "vmess",
			Raw:         "vmess://eyJ2IjoiMiIsInBzIjoiRGlja3kncyBTZXJ2ZXIifQ==",
		},
		{
			Server:      "sg-server.net",
			IP:          "104.18.2.2",
			ServerPort:  80,
			Password:    "secret'pass",
			Method:      "aes-128-gcm",
			Transport:   "tcp",
			Remark:      "[2] 🇸🇬 Singapore Shadowsocks",
			ConnMode:    "direct",
			CountryCode: "SG",
			Region:      "SINGAPORE",
			Org:         "Cloudflare, Inc.",
			VPN:         "shadowsocks",
			Raw:         "ss://YWVzLTEyOC1nY206c2VjcmV0QHNnLXNlcnZlci5uZXQ6ODAjU0c=",
		},
	}

	t.Run("SaveBatch with clearFirst = true", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM proxies;")).WillReturnResult(sqlmock.NewResult(0, 0))

		prep := mock.ExpectPrepare(regexp.QuoteMeta(insertProxyQuery))
		// nodes[0] has ConnMode "cdn,sni" -> expanded into 2 separate rows: cdn and sni
		prep.ExpectExec().
			WithArgs(
				nodes[0].Server,
				nodes[0].IP,
				nodes[0].ServerPort,
				nodes[0].UUID,
				nodes[0].Password,
				nodes[0].Security,
				nodes[0].AlterID,
				nodes[0].Method,
				nodes[0].Plugin,
				nodes[0].PluginOpts,
				nodes[0].Host,
				1, // TLS = true => 1
				nodes[0].Transport,
				nodes[0].Path,
				nodes[0].ServiceName,
				0, // Insecure = false => 0
				nodes[0].SNI,
				"[1] 🇮🇩 Dicky's Fast Server WS CDN TLS",
				"cdn",
				nodes[0].CountryCode,
				nodes[0].Region,
				nodes[0].Org,
				nodes[0].VPN,
				nodes[0].Raw,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		prep.ExpectExec().
			WithArgs(
				nodes[0].Server,
				nodes[0].IP,
				nodes[0].ServerPort,
				nodes[0].UUID,
				nodes[0].Password,
				nodes[0].Security,
				nodes[0].AlterID,
				nodes[0].Method,
				nodes[0].Plugin,
				nodes[0].PluginOpts,
				nodes[0].Host,
				1, // TLS = true => 1
				nodes[0].Transport,
				nodes[0].Path,
				nodes[0].ServiceName,
				0, // Insecure = false => 0
				nodes[0].SNI,
				"[1] 🇮🇩 Dicky's Fast Server WS SNI TLS",
				"sni",
				nodes[0].CountryCode,
				nodes[0].Region,
				nodes[0].Org,
				nodes[0].VPN,
				nodes[0].Raw,
			).
			WillReturnResult(sqlmock.NewResult(2, 1))

		// nodes[1] has ConnMode "direct" -> single row
		prep.ExpectExec().
			WithArgs(
				nodes[1].Server,
				nodes[1].IP,
				nodes[1].ServerPort,
				nodes[1].UUID,
				nodes[1].Password,
				nodes[1].Security,
				nodes[1].AlterID,
				nodes[1].Method,
				nodes[1].Plugin,
				nodes[1].PluginOpts,
				nodes[1].Host,
				0, // TLS = false => 0
				nodes[1].Transport,
				nodes[1].Path,
				nodes[1].ServiceName,
				0,
				nodes[1].SNI,
				nodes[1].Remark,
				nodes[1].ConnMode,
				nodes[1].CountryCode,
				nodes[1].Region,
				nodes[1].Org,
				nodes[1].VPN,
				nodes[1].Raw,
			).
			WillReturnResult(sqlmock.NewResult(3, 1))

		mock.ExpectCommit()

		inserted, err := storageDB.SaveBatch(ctx, nodes, true)
		require.NoError(t, err)
		assert.Equal(t, 3, inserted)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SaveBatch with empty slice", func(t *testing.T) {
		inserted, err := storageDB.SaveBatch(ctx, nil, false)
		require.NoError(t, err)
		assert.Equal(t, 0, inserted)
	})
}

func TestDatabase_Count_And_GetAll(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	storageDB, err := NewDatabase(mockDB)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Count proxies", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM proxies;")).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

		count, err := storageDB.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 42, count)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetAll proxies", func(t *testing.T) {
		columns := []string{
			"id", "server", "ip", "server_port", "uuid", "password", "security", "alter_id",
			"method", "plugin", "plugin_opts", "host", "tls", "transport", "path", "service_name",
			"insecure", "sni", "remark", "conn_mode", "country_code", "region", "org", "vpn", "raw",
		}

		rows := sqlmock.NewRows(columns).
			AddRow(
				1, "id-server.net", "103.1.2.3", 443, "uuid-1", "", "auto", 0,
				"", "", "", "id-server.net", 1, "ws", "/ws", "",
				0, "id-server.net", "[1] 🇮🇩 Test VMess", "cdn", "ID", "JAKARTA", "Telkom", "vmess", "vmess://raw1",
			).
			AddRow(
				2, "sg-server.net", "104.18.2.2", 8388, "", "pass-2", "", 0,
				"aes-128-gcm", "", "", "", 0, "tcp", "", "",
				0, "", "[2] 🇸🇬 Test SS", "direct", "SG", "SINGAPORE", "Cloudflare", "shadowsocks", "ss://raw2",
			)

		expectedQuery := `SELECT id, server, ip, server_port, uuid, password, security, alter_id,
		method, plugin, plugin_opts, host, tls, transport, path, service_name,
		insecure, sni, remark, conn_mode, country_code, region, org, vpn, raw
		FROM proxies ORDER BY id ASC;`

		mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).WillReturnRows(rows)

		all, err := storageDB.GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, all, 2)

		assert.Equal(t, "id-server.net", all[0].Server)
		assert.True(t, all[0].TLS)
		assert.False(t, all[0].Insecure)
		assert.Equal(t, "vmess", all[0].VPN)

		assert.Equal(t, "sg-server.net", all[1].Server)
		assert.False(t, all[1].TLS)
		assert.Equal(t, "shadowsocks", all[1].VPN)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Ping and Close", func(t *testing.T) {
		pDB, pMock, pErr := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, pErr)
		defer pDB.Close()

		pStorage, pErr := NewDatabase(pDB)
		require.NoError(t, pErr)

		pMock.ExpectPing()
		pMock.ExpectClose()

		require.NoError(t, pStorage.Ping(ctx))
		require.NoError(t, pStorage.Close())
		assert.NoError(t, pMock.ExpectationsWereMet())
	})
}

func TestExporter_ExportAll(t *testing.T) {
	tempDir := t.TempDir()
	exporter := NewExporter(tempDir)
	assert.Equal(t, tempDir, exporter.OutputDir())

	nodes := []*model.ProxyNode{
		{
			Server:      "1.2.3.4",
			ServerPort:  443,
			VPN:         "vmess",
			UUID:        "00000000-0000-0000-0000-000000000001",
			Transport:   "ws",
			Host:        "host.com",
			Path:        "/ws",
			TLS:         true,
			Remark:      "Test VMess",
			Raw:         "vmess://test-raw-uri-1",
		},
		{
			Server:      "5.6.7.8",
			ServerPort:  443,
			VPN:         "trojan",
			Password:    "trojan-password",
			Transport:   "ws",
			Host:        "trojan.com",
			TLS:         true,
			Remark:      "Test Trojan",
			Raw:         "trojan://test-raw-uri-2",
		},
	}

	err := exporter.ExportAll(nodes)
	require.NoError(t, err)

	// 1. Verify 'nodes' file
	rawPath := filepath.Join(tempDir, "nodes")
	rawBytes, err := os.ReadFile(rawPath)
	require.NoError(t, err)
	rawStr := string(rawBytes)
	assert.Contains(t, rawStr, "vmess://test-raw-uri-1")
	assert.Contains(t, rawStr, "trojan://test-raw-uri-2")

	// 2. Verify 'sub' file (Base64)
	subPath := filepath.Join(tempDir, "sub")
	subBytes, err := os.ReadFile(subPath)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(subBytes)))
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "vmess://test-raw-uri-1")

	// 3. Verify 'singbox.json' file
	sbPath := filepath.Join(tempDir, "singbox.json")
	sbBytes, err := os.ReadFile(sbPath)
	require.NoError(t, err)
	var sbConfig map[string]any
	err = json.Unmarshal(sbBytes, &sbConfig)
	require.NoError(t, err, "singbox.json must be valid JSON")
	assert.Contains(t, sbConfig, "outbounds")
	assert.Contains(t, sbConfig, "inbounds")

	// 4. Verify 'clash.yaml' file
	clashPath := filepath.Join(tempDir, "clash.yaml")
	clashBytes, err := os.ReadFile(clashPath)
	require.NoError(t, err)
	var clashConfig map[string]any
	err = yaml.Unmarshal(clashBytes, &clashConfig)
	require.NoError(t, err, "clash.yaml must be valid YAML")
	assert.Contains(t, clashConfig, "proxies")
	assert.Contains(t, clashConfig, "proxy-groups")
}
