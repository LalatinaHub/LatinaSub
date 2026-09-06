package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/LalatinaHub/LatinaSub/pkg/logger"
	"github.com/LalatinaHub/common/database"
	"github.com/LalatinaHub/common/model"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

const (
	createTableQuery = `CREATE TABLE IF NOT EXISTS proxies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server TEXT NOT NULL,
		ip TEXT,
		server_port INTEGER NOT NULL,
		uuid TEXT,
		password TEXT,
		security TEXT,
		alter_id INTEGER DEFAULT 0,
		method TEXT,
		plugin TEXT,
		plugin_opts TEXT,
		host TEXT,
		tls INTEGER DEFAULT 0,
		transport TEXT,
		path TEXT,
		service_name TEXT,
		insecure INTEGER DEFAULT 0,
		sni TEXT,
		remark TEXT,
		conn_mode TEXT,
		country_code TEXT,
		region TEXT,
		org TEXT,
		vpn TEXT NOT NULL,
		raw TEXT
	);`

	createIndexQuery = `CREATE INDEX IF NOT EXISTS idx_proxies_filter ON proxies(vpn, country_code, region, conn_mode);`
	createIndexCC    = `CREATE INDEX IF NOT EXISTS idx_proxies_country ON proxies(country_code);`

	insertProxyQuery = `INSERT INTO proxies (
		server, ip, server_port, uuid, password, security, alter_id,
		method, plugin, plugin_opts, host, tls, transport, path,
		service_name, insecure, sni, remark, conn_mode, country_code,
		region, org, vpn, raw
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
)

// Database manages persistent storage of proxy nodes in Turso LibSQL.
type Database struct {
	db *sql.DB
}

// NewDatabase initializes a Database instance.
// If db is nil, the shared singleton pool from common/database is used.
func NewDatabase(db *sql.DB) (*Database, error) {
	if db == nil {
		var err error
		db, err = database.GetDB()
		if err != nil {
			return nil, fmt.Errorf("failed to get shared database connection: %w", err)
		}
	}
	return &Database{db: db}, nil
}

// InitSchema ensures that the proxies table and necessary performance indexes exist.
func (d *Database) InitSchema(ctx context.Context) error {
	if _, err := d.db.ExecContext(ctx, createTableQuery); err != nil {
		return fmt.Errorf("failed to create proxies table: %w", err)
	}

	if _, err := d.db.ExecContext(ctx, createIndexQuery); err != nil {
		return fmt.Errorf("failed to create filter index: %w", err)
	}

	if _, err := d.db.ExecContext(ctx, createIndexCC); err != nil {
		return fmt.Errorf("failed to create country code index: %w", err)
	}

	logger.Info().Msg("Database proxies table and indexes verified")
	return nil
}

// ExpandNodesByMode splits any node having a comma-separated ConnMode (e.g. "cdn,sni")
// into multiple individual nodes, each with a single normalized ConnMode (e.g. "cdn", "sni").
// It ensures that conn_mode saved to the database never contains commas.
func ExpandNodesByMode(nodes []*model.ProxyNode) []*model.ProxyNode {
	expanded := make([]*model.ProxyNode, 0, len(nodes))

	for _, node := range nodes {
		if node == nil {
			continue
		}

		rawModes := strings.Split(node.ConnMode, ",")
		cleanModes := make([]string, 0, len(rawModes))
		for _, m := range rawModes {
			m = strings.ToLower(strings.TrimSpace(m))
			if m != "" {
				cleanModes = append(cleanModes, m)
			}
		}

		if len(cleanModes) <= 1 {
			clone := *node
			if len(cleanModes) == 1 {
				clone.ConnMode = cleanModes[0]
			} else {
				clone.ConnMode = ""
			}
			expanded = append(expanded, &clone)
			continue
		}

		// Node supports multiple modes: split into separate accounts per mode
		for _, mode := range cleanModes {
			clone := *node
			clone.ConnMode = mode

			// Update remark to show the single mode instead of other/combined modes
			if clone.Remark != "" {
				upperSingleMode := strings.ToUpper(mode)
				if node.ConnMode != "" && strings.Contains(clone.Remark, strings.ToUpper(node.ConnMode)) {
					clone.Remark = strings.Replace(clone.Remark, strings.ToUpper(node.ConnMode), upperSingleMode, 1)
				} else {
					for _, other := range cleanModes {
						if other != mode && strings.Contains(clone.Remark, " "+strings.ToUpper(other)+" ") {
							clone.Remark = strings.Replace(clone.Remark, " "+strings.ToUpper(other)+" ", " "+upperSingleMode+" ", 1)
							break
						}
					}
				}
			}

			expanded = append(expanded, &clone)
		}
	}

	return expanded
}

// SaveBatch commits a slice of ProxyNode models into the database using parameterized prepared statements.
// If clearFirst is true, all existing proxies are purged before insertion within the same transaction.
// Any node with comma-separated ConnMode (e.g. "cdn,sni") is automatically expanded into separate rows.
func (d *Database) SaveBatch(ctx context.Context, nodes []*model.ProxyNode, clearFirst bool) (int, error) {
	if len(nodes) == 0 {
		return 0, nil
	}

	nodes = ExpandNodesByMode(nodes)

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if clearFirst {
		if _, err := tx.ExecContext(ctx, "DELETE FROM proxies;"); err != nil {
			return 0, fmt.Errorf("failed to truncate proxies table: %w", err)
		}
	}

	stmt, err := tx.PrepareContext(ctx, insertProxyQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	var count int
	for _, node := range nodes {
		if node == nil {
			continue
		}

		tlsVal := 0
		if node.TLS {
			tlsVal = 1
		}

		insecureVal := 0
		if node.Insecure {
			insecureVal = 1
		}

		_, err := stmt.ExecContext(
			ctx,
			node.Server,
			node.IP,
			node.ServerPort,
			node.UUID,
			node.Password,
			node.Security,
			node.AlterID,
			node.Method,
			node.Plugin,
			node.PluginOpts,
			node.Host,
			tlsVal,
			node.Transport,
			node.Path,
			node.ServiceName,
			insecureVal,
			node.SNI,
			node.Remark,
			node.ConnMode,
			node.CountryCode,
			node.Region,
			node.Org,
			node.VPN,
			node.Raw,
		)
		if err != nil {
			return count, fmt.Errorf("failed to insert proxy node %s:%d: %w", node.Server, node.ServerPort, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info().
		Int("inserted", count).
		Msg("Successfully persisted proxy batch to database")

	return count, nil
}

// Count returns the current total number of proxies stored in the database.
func (d *Database) Count(ctx context.Context) (int, error) {
	var count int
	row := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM proxies;")
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count proxies: %w", err)
	}
	return count, nil
}

// GetAll retrieves all stored proxies from the database.
func (d *Database) GetAll(ctx context.Context) ([]model.ProxyNode, error) {
	query := `SELECT id, server, ip, server_port, uuid, password, security, alter_id,
		method, plugin, plugin_opts, host, tls, transport, path, service_name,
		insecure, sni, remark, conn_mode, country_code, region, org, vpn, raw
		FROM proxies ORDER BY id ASC;`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all proxies: %w", err)
	}
	defer rows.Close()

	var nodes []model.ProxyNode
	for rows.Next() {
		var (
			p        model.ProxyNode
			tlsVal   int
			insecVal int
		)

		err := rows.Scan(
			&p.ID,
			&p.Server,
			&p.IP,
			&p.ServerPort,
			&p.UUID,
			&p.Password,
			&p.Security,
			&p.AlterID,
			&p.Method,
			&p.Plugin,
			&p.PluginOpts,
			&p.Host,
			&tlsVal,
			&p.Transport,
			&p.Path,
			&p.ServiceName,
			&insecVal,
			&p.SNI,
			&p.Remark,
			&p.ConnMode,
			&p.CountryCode,
			&p.Region,
			&p.Org,
			&p.VPN,
			&p.Raw,
		)
		if err != nil {
			continue
		}

		p.TLS = (tlsVal == 1)
		p.Insecure = (insecVal == 1)
		nodes = append(nodes, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during proxy rows iteration: %w", err)
	}

	return nodes, nil
}

// Ping checks whether the database connection is alive.
func (d *Database) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// Close closes the underlying database pool.
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}
