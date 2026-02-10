package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// KeyRecord represents a stored API key
type KeyRecord struct {
	Name             string    `json:"name"`
	Key              string    `json:"key"`
	AllowedUpstreams string    `json:"allowed_upstreams"`
	Policy           string    `json:"policy"`
	CreatedAt        time.Time `json:"created_at"`
	LastUsedAt       time.Time `json:"last_used_at"`
	RequestCount     int64     `json:"request_count"`
}

// DB handles SQLite persistence
type DB struct {
	conn *sql.DB
}

// New creates a new database connection
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS api_keys (
		name TEXT PRIMARY KEY,
		key TEXT UNIQUE NOT NULL,
		allowed_upstreams TEXT,
		policy TEXT DEFAULT 'default',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		request_count INTEGER DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_key ON api_keys(key);
	`

	_, err := db.conn.Exec(query)
	return err
}

// SaveKey saves or updates an API key
func (db *DB) SaveKey(key *KeyRecord) error {
	query := `
	INSERT INTO api_keys (name, key, allowed_upstreams, policy, created_at, last_used_at, request_count)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		key = excluded.key,
		allowed_upstreams = excluded.allowed_upstreams,
		policy = excluded.policy,
		last_used_at = excluded.last_used_at,
		request_count = excluded.request_count
	`

	_, err := db.conn.Exec(query,
		key.Name,
		key.Key,
		key.AllowedUpstreams,
		key.Policy,
		key.CreatedAt,
		key.LastUsedAt,
		key.RequestCount,
	)
	return err
}

// GetKey retrieves a key by its name
func (db *DB) GetKey(name string) (*KeyRecord, error) {
	query := `SELECT name, key, allowed_upstreams, policy, created_at, last_used_at, request_count 
	          FROM api_keys WHERE name = ?`

	row := db.conn.QueryRow(query, name)
	return db.scanKey(row)
}

// GetKeyByValue retrieves a key by its key value
func (db *DB) GetKeyByValue(keyValue string) (*KeyRecord, error) {
	query := `SELECT name, key, allowed_upstreams, policy, created_at, last_used_at, request_count 
	          FROM api_keys WHERE key = ?`

	row := db.conn.QueryRow(query, keyValue)
	return db.scanKey(row)
}

// GetAllKeys retrieves all keys
func (db *DB) GetAllKeys() ([]*KeyRecord, error) {
	query := `SELECT name, key, allowed_upstreams, policy, created_at, last_used_at, request_count 
	          FROM api_keys ORDER BY created_at DESC`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*KeyRecord
	for rows.Next() {
		key, err := db.scanKeyFromRows(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// DeleteKey deletes a key by name
func (db *DB) DeleteKey(name string) error {
	query := `DELETE FROM api_keys WHERE name = ?`
	_, err := db.conn.Exec(query, name)
	return err
}

// UpdateUsage updates the usage statistics for a key
func (db *DB) UpdateUsage(name string) error {
	query := `UPDATE api_keys SET 
	          request_count = request_count + 1, 
	          last_used_at = CURRENT_TIMESTAMP 
	          WHERE name = ?`
	_, err := db.conn.Exec(query, name)
	return err
}

// scanKey scans a single key from a row
func (db *DB) scanKey(row *sql.Row) (*KeyRecord, error) {
	var key KeyRecord
	var lastUsed sql.NullTime

	err := row.Scan(
		&key.Name,
		&key.Key,
		&key.AllowedUpstreams,
		&key.Policy,
		&key.CreatedAt,
		&lastUsed,
		&key.RequestCount,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if lastUsed.Valid {
		key.LastUsedAt = lastUsed.Time
	}

	return &key, err
}

// scanKeyFromRows scans a key from active rows
func (db *DB) scanKeyFromRows(rows *sql.Rows) (*KeyRecord, error) {
	var key KeyRecord
	var lastUsed sql.NullTime

	err := rows.Scan(
		&key.Name,
		&key.Key,
		&key.AllowedUpstreams,
		&key.Policy,
		&key.CreatedAt,
		&lastUsed,
		&key.RequestCount,
	)

	if lastUsed.Valid {
		key.LastUsedAt = lastUsed.Time
	}

	return &key, err
}

// GetDBPath returns the default database path
func GetDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "modeltunnel.db"
	}
	return filepath.Join(home, ".config", "modeltunnel", "keys.db")
}
