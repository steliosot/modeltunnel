package keys

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modeltunnel/modeltunnel/internal/db"
)

// Key represents an API key
type Key struct {
	Name             string    `json:"name"`
	Key              string    `json:"key"`
	AllowedUpstreams []string  `json:"allowed_upstreams"`
	Policy           string    `json:"policy"`
	CreatedAt        time.Time `json:"created_at"`
	LastUsedAt       time.Time `json:"last_used_at,omitempty"`
	RequestCount     int64     `json:"request_count"`
}

// Store manages API keys
type Store struct {
	mu   sync.RWMutex
	keys map[string]*Key
	db   *db.DB
}

// NewStore creates a new key store
func NewStore() *Store {
	return &Store{
		keys: make(map[string]*Key),
	}
}

// NewStoreWithDB creates a new key store with database persistence
func NewStoreWithDB(database *db.DB) *Store {
	s := &Store{
		keys: make(map[string]*Key),
		db:   database,
	}

	// Load existing keys from database
	if err := s.loadFromDB(); err != nil {
		fmt.Printf("⚠️  Warning: %v\n", err)
		fmt.Println("   Starting with empty key store. You may need to recreate your keys.")
	}

	// Start background key refresh goroutine
	go s.startKeyRefresher()

	return s
}

// startKeyRefresher periodically reloads keys from database
// This ensures keys created via CLI are available immediately
func (s *Store) startKeyRefresher() {
	if s.db == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := s.refreshFromDB(); err != nil {
			// Log error but do not crash - we'll retry on next tick
			continue
		}
	}
}

// refreshFromDB reloads keys from DB, adding new ones and updating existing
// Unlike loadFromDB, this preserves in-memory usage stats
func (s *Store) refreshFromDB() error {
	if s.db == nil {
		return nil
	}

	records, err := s.db.GetAllKeys()
	if err != nil {
		return fmt.Errorf("failed to refresh keys from database: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track which keys we've seen
	seenKeys := make(map[string]bool)

	for _, record := range records {
		seenKeys[record.Key] = true

		// Check if key already exists in memory
		if existingKey, ok := s.keys[record.Key]; ok {
			// Update mutable fields but preserve usage stats
			existingKey.Name = record.Name
			existingKey.AllowedUpstreams = parseUpstreams(record.AllowedUpstreams)
			existingKey.Policy = record.Policy
			// Note: We preserve existingKey.RequestCount and existingKey.LastUsedAt
		} else {
			// New key - add it
			key := &Key{
				Name:             record.Name,
				Key:              record.Key,
				AllowedUpstreams: parseUpstreams(record.AllowedUpstreams),
				Policy:           record.Policy,
				CreatedAt:        record.CreatedAt,
				LastUsedAt:       record.LastUsedAt,
				RequestCount:     record.RequestCount,
			}
			s.keys[key.Key] = key
		}
	}

	// Remove keys that no longer exist in DB (were revoked)
	for keyValue, key := range s.keys {
		if !seenKeys[keyValue] {
			delete(s.keys, keyValue)
			fmt.Printf("🗑️  Key '%s' removed (revoked via CLI)\n", key.Name)
		}
	}

	return nil
}

// loadFromDB loads keys from the database
func (s *Store) loadFromDB() error {
	if s.db == nil {
		return fmt.Errorf("no database connection")
	}

	records, err := s.db.GetAllKeys()
	if err != nil {
		return fmt.Errorf("failed to load keys from database: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	loadedCount := 0
	for _, record := range records {
		key := &Key{
			Name:             record.Name,
			Key:              record.Key,
			AllowedUpstreams: parseUpstreams(record.AllowedUpstreams),
			Policy:           record.Policy,
			CreatedAt:        record.CreatedAt,
			LastUsedAt:       record.LastUsedAt,
			RequestCount:     record.RequestCount,
		}
		s.keys[key.Key] = key
		loadedCount++
	}

	return nil
}

// parseUpstreams parses a comma-separated string into a slice
func parseUpstreams(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// joinUpstreams joins a slice into a comma-separated string
func joinUpstreams(upstreams []string) string {
	if len(upstreams) == 0 {
		return ""
	}
	return strings.Join(upstreams, ",")
}

// GenerateKey generates a new API key
func GenerateKey(name string) string {
	prefix := "mt_sk_" + name + "_"
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return prefix + hex.EncodeToString(bytes)
}

// Create creates a new key
func (s *Store) Create(name string, allowedUpstreams []string, policy string) *Key {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing key with same name
	for k, v := range s.keys {
		if v.Name == name {
			delete(s.keys, k)
			// Also delete from DB
			if s.db != nil {
				s.db.DeleteKey(name)
			}
		}
	}

	key := &Key{
		Name:             name,
		Key:              GenerateKey(name),
		AllowedUpstreams: allowedUpstreams,
		Policy:           policy,
		CreatedAt:        time.Now(),
	}

	s.keys[key.Key] = key

	// Persist to database
	if s.db != nil {
		if err := s.db.SaveKey(&db.KeyRecord{
			Name:             key.Name,
			Key:              key.Key,
			AllowedUpstreams: joinUpstreams(key.AllowedUpstreams),
			Policy:           key.Policy,
			CreatedAt:        key.CreatedAt,
			LastUsedAt:       key.LastUsedAt,
			RequestCount:     key.RequestCount,
		}); err != nil {
			fmt.Printf("⚠️  Warning: Failed to save key '%s' to database: %v\n", key.Name, err)
		}
	}

	return key
}

// Get retrieves a key by its value
func (s *Store) Get(keyValue string) (*Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[keyValue]
	return key, ok
}

// GetByName retrieves a key by its name
func (s *Store) GetByName(name string) (*Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.keys {
		if key.Name == name {
			return key, true
		}
	}
	return nil, false
}

// List returns all keys
func (s *Store) List() []*Key {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]*Key, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, key)
	}
	return keys
}

// Revoke removes a key by name
func (s *Store) Revoke(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range s.keys {
		if v.Name == name {
			delete(s.keys, k)
			// Also delete from DB
			if s.db != nil {
				if err := s.db.DeleteKey(name); err != nil {
					fmt.Printf("⚠️  Warning: Failed to delete key '%s' from database: %v\n", name, err)
				}
			}
			return true
		}
	}
	return false
}

// RecordUsage records usage for a key
func (s *Store) RecordUsage(keyValue string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok := s.keys[keyValue]; ok {
		key.LastUsedAt = time.Now()
		key.RequestCount++

		// Update in database
		if s.db != nil {
			if err := s.db.UpdateUsage(key.Name); err != nil {
				// Don't print warning for every request to avoid log spam
				// Just update in-memory and continue
			}
		}
	}
}

// CanAccessUpstream checks if a key can access a specific upstream
func (k *Key) CanAccessUpstream(upstream string) bool {
	if len(k.AllowedUpstreams) == 0 {
		return true
	}
	for _, u := range k.AllowedUpstreams {
		if u == upstream {
			return true
		}
	}
	return false
}

// KeyConfig represents a key configuration for reloading
type KeyConfig struct {
	Name             string   `yaml:"name"`
	Key              string   `yaml:"key"`
	AllowedUpstreams []string `yaml:"allowed_upstreams"`
	Policy           string   `yaml:"policy"`
}

// ReloadFromConfig reloads keys from config while preserving usage stats
// Note: When using DB persistence, this is only used for initial migration
func (s *Store) ReloadFromConfig(configKeys []KeyConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we have a DB, only add keys that don't already exist
	if s.db != nil {
		for _, k := range configKeys {
			// Check if key already exists in DB
			existing, _ := s.db.GetKey(k.Name)
			if existing != nil {
				continue // Skip existing keys
			}

			key := &Key{
				Name:             k.Name,
				Key:              k.Key,
				AllowedUpstreams: k.AllowedUpstreams,
				Policy:           k.Policy,
				CreatedAt:        time.Now(),
			}
			s.keys[key.Key] = key

			// Save to DB
			s.db.SaveKey(&db.KeyRecord{
				Name:             key.Name,
				Key:              key.Key,
				AllowedUpstreams: joinUpstreams(key.AllowedUpstreams),
				Policy:           key.Policy,
				CreatedAt:        key.CreatedAt,
			})
		}
		return
	}

	// Create a map of existing keys to preserve usage stats
	existingStats := make(map[string]struct {
		lastUsedAt   time.Time
		requestCount int64
	})
	for _, key := range s.keys {
		existingStats[key.Name] = struct {
			lastUsedAt   time.Time
			requestCount int64
		}{
			lastUsedAt:   key.LastUsedAt,
			requestCount: key.RequestCount,
		}
	}

	// Clear and reload keys
	s.keys = make(map[string]*Key)
	for _, k := range configKeys {
		key := &Key{
			Name:             k.Name,
			Key:              k.Key,
			AllowedUpstreams: k.AllowedUpstreams,
			Policy:           k.Policy,
			CreatedAt:        time.Now(),
		}

		// Preserve usage stats if key existed before
		if stats, ok := existingStats[k.Name]; ok {
			key.LastUsedAt = stats.lastUsedAt
			key.RequestCount = stats.requestCount
		}

		s.keys[key.Key] = key
	}
}

// HasDB returns true if the store is using database persistence
func (s *Store) HasDB() bool {
	return s.db != nil
}
