package providers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Provider represents an external API provider (OpenAI, Anthropic, etc.)
type Provider struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`           // Display name (e.g., "My OpenAI Key")
	Type          string    `json:"type"`           // Provider type: "openai", "anthropic"
	APIKey        string    `json:"-"`              // Decrypted API key (not serialized)
	APIKeyMasked  string    `json:"api_key_masked"` // Masked key for display
	BaseURL       string    `json:"base_url"`       // API base URL
	Models        []string  `json:"models"`         // Available models
	RateLimit     string    `json:"rate_limit"`     // e.g., "100/min"
	Priority      int       `json:"priority"`       // Failover priority (lower = higher priority)
	IsActive      bool      `json:"is_active"`
	TrackCosts    bool      `json:"track_costs"` // Enable cost tracking
	TotalRequests int64     `json:"total_requests"`
	TotalTokens   int64     `json:"total_tokens"`
	TotalCost     float64   `json:"total_cost"` // Estimated cost in USD
	CreatedAt     time.Time `json:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
}

// ProviderStore manages provider data in SQLite
type ProviderStore struct {
	db *sql.DB
}

// NewProviderStore creates a new provider store
func NewProviderStore(db *sql.DB) (*ProviderStore, error) {
	store := &ProviderStore{db: db}
	if err := store.createTable(); err != nil {
		return nil, fmt.Errorf("create providers table: %w", err)
	}
	return store, nil
}

// createTable creates the providers table if it doesn't exist
func (s *ProviderStore) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS providers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		api_key_encrypted TEXT NOT NULL,
		base_url TEXT,
		models TEXT, -- JSON array
		rate_limit TEXT,
		priority INTEGER DEFAULT 0,
		is_active BOOLEAN DEFAULT 1,
		track_costs BOOLEAN DEFAULT 1,
		total_requests INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		total_cost REAL DEFAULT 0.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME
	)`

	_, err := s.db.Exec(query)
	return err
}

// Create adds a new provider
func (s *ProviderStore) Create(provider *Provider) error {
	// Encrypt API key
	encryptedKey, err := Encrypt(provider.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	// Serialize models to JSON
	modelsJSON, err := json.Marshal(provider.Models)
	if err != nil {
		return fmt.Errorf("marshal models: %w", err)
	}

	query := `
		INSERT INTO providers (
			id, name, type, api_key_encrypted, base_url, models, 
			rate_limit, priority, is_active, track_costs, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		provider.ID,
		provider.Name,
		provider.Type,
		encryptedKey,
		provider.BaseURL,
		string(modelsJSON),
		provider.RateLimit,
		provider.Priority,
		provider.IsActive,
		provider.TrackCosts,
		time.Now(),
	)

	return err
}

// Get retrieves a provider by ID
func (s *ProviderStore) Get(id string) (*Provider, error) {
	query := `
		SELECT id, name, type, api_key_encrypted, base_url, models,
		       rate_limit, priority, is_active, track_costs,
		       total_requests, total_tokens, total_cost, created_at, last_used_at
		FROM providers WHERE id = ?`

	row := s.db.QueryRow(query, id)
	return s.scanProvider(row)
}

// List retrieves all providers
func (s *ProviderStore) List() ([]*Provider, error) {
	query := `
		SELECT id, name, type, api_key_encrypted, base_url, models,
		       rate_limit, priority, is_active, track_costs,
		       total_requests, total_tokens, total_cost, created_at, last_used_at
		FROM providers ORDER BY priority ASC, created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		provider, err := s.scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

// ListActive retrieves only active providers
func (s *ProviderStore) ListActive() ([]*Provider, error) {
	query := `
		SELECT id, name, type, api_key_encrypted, base_url, models,
		       rate_limit, priority, is_active, track_costs,
		       total_requests, total_tokens, total_cost, created_at, last_used_at
		FROM providers WHERE is_active = 1 ORDER BY priority ASC, created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		provider, err := s.scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

// Update modifies a provider
func (s *ProviderStore) Update(provider *Provider) error {
	// Encrypt API key if provided
	var encryptedKey interface{}
	if provider.APIKey != "" {
		key, err := Encrypt(provider.APIKey)
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		encryptedKey = key
	} else {
		encryptedKey = nil
	}

	// Serialize models to JSON
	modelsJSON, err := json.Marshal(provider.Models)
	if err != nil {
		return fmt.Errorf("marshal models: %w", err)
	}

	query := `
		UPDATE providers SET
			name = ?,
			type = ?,
			api_key_encrypted = COALESCE(?, api_key_encrypted),
			base_url = ?,
			models = ?,
			rate_limit = ?,
			priority = ?,
			is_active = ?,
			track_costs = ?
		WHERE id = ?`

	_, err = s.db.Exec(query,
		provider.Name,
		provider.Type,
		encryptedKey,
		provider.BaseURL,
		string(modelsJSON),
		provider.RateLimit,
		provider.Priority,
		provider.IsActive,
		provider.TrackCosts,
		provider.ID,
	)

	return err
}

// Delete removes a provider
func (s *ProviderStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM providers WHERE id = ?", id)
	return err
}

// UpdateUsage updates provider usage statistics
func (s *ProviderStore) UpdateUsage(id string, requests, tokens int64, cost float64) error {
	query := `
		UPDATE providers SET
			total_requests = total_requests + ?,
			total_tokens = total_tokens + ?,
			total_cost = total_cost + ?,
			last_used_at = ?
		WHERE id = ?`

	_, err := s.db.Exec(query, requests, tokens, cost, time.Now(), id)
	return err
}

// UpdateLastUsed updates the last used timestamp
func (s *ProviderStore) UpdateLastUsed(id string) error {
	_, err := s.db.Exec("UPDATE providers SET last_used_at = ? WHERE id = ?", time.Now(), id)
	return err
}

// scanProvider scans a database row into a Provider struct
func (s *ProviderStore) scanProvider(scanner interface {
	Scan(dest ...interface{}) error
}) (*Provider, error) {
	var p Provider
	var modelsJSON string
	var encryptedKey string

	err := scanner.Scan(
		&p.ID,
		&p.Name,
		&p.Type,
		&encryptedKey,
		&p.BaseURL,
		&modelsJSON,
		&p.RateLimit,
		&p.Priority,
		&p.IsActive,
		&p.TrackCosts,
		&p.TotalRequests,
		&p.TotalTokens,
		&p.TotalCost,
		&p.CreatedAt,
		&p.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}

	// Decrypt API key
	decryptedKey, err := Decrypt(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}
	p.APIKey = decryptedKey
	p.APIKeyMasked = MaskKey(decryptedKey)

	// Deserialize models
	if err := json.Unmarshal([]byte(modelsJSON), &p.Models); err != nil {
		return nil, fmt.Errorf("unmarshal models: %w", err)
	}

	return &p, nil
}
