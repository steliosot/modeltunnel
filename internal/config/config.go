package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderConfig represents external API provider configuration in config
type ProviderConfig struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	APIKey    string   `yaml:"api_key"`
	BaseURL   string   `yaml:"base_url,omitempty"`
	Models    []string `yaml:"models,omitempty"`
	RateLimit string   `yaml:"rate_limit,omitempty"`
	Priority  int      `yaml:"priority,omitempty"`
}

// Config represents the modeltunnel configuration
type Config struct {
	Server    ServerConfig        `yaml:"server"`
	Upstreams map[string]Upstream `yaml:"upstreams"`
	Policies  map[string]Policy   `yaml:"policies"`
	Keys      []KeyConfig         `yaml:"keys"`
	Intents   map[string]Intent   `yaml:"intents,omitempty"`
	Providers []ProviderConfig    `yaml:"providers,omitempty"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Host  string      `yaml:"host"`
	Port  int         `yaml:"port"`
	Admin AdminConfig `yaml:"admin,omitempty"`
}

// AdminConfig represents admin panel authentication
type AdminConfig struct {
	Enabled  bool   `yaml:"enabled"`  // Enable admin panel access via tunnel
	Username string `yaml:"username"` // Basic auth username
	Password string `yaml:"password"` // Basic auth password
}

// AdminConfig represents admin panel authentication
type AdminConfig struct {
	Enabled  bool   `yaml:"enabled"`  // Enable admin panel access via tunnel
	Username string `yaml:"username"` // Basic auth username
	Password string `yaml:"password"` // Basic auth password
}

// Upstream represents an upstream provider
type Upstream struct {
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
}

// Policy represents a usage policy
type Policy struct {
	RateLimit     string                 `yaml:"rate_limit"`
	MaxTokens     int                    `yaml:"max_tokens"`
	AllowedModels []string               `yaml:"allowed_models,omitempty"`
	Models        map[string]ModelPolicy `yaml:"models,omitempty"` // Per-model overrides
}

// ModelPolicy represents policy overrides for a specific model
type ModelPolicy struct {
	RateLimit string `yaml:"rate_limit,omitempty"`
	MaxTokens int    `yaml:"max_tokens,omitempty"`
}

// Intent represents an intent-based routing configuration
type Intent struct {
	Priority    []string `yaml:"priority"`    // Ordered list of preferred models
	Description string   `yaml:"description"` // Description of this intent
	Temperature float32  `yaml:"temperature"` // Default temperature for this intent
	MaxTokens   int      `yaml:"max_tokens"`  // Default max_tokens for this intent
}

// GetEffectivePolicy returns the effective policy for a specific model
// It merges default policy with model-specific overrides
func (p Policy) GetEffectivePolicy(modelName string) Policy {
	result := Policy{
		RateLimit:     p.RateLimit,
		MaxTokens:     p.MaxTokens,
		AllowedModels: p.AllowedModels,
	}

	if p.Models == nil {
		return result
	}

	// Try exact match first
	if modelPolicy, ok := p.Models[modelName]; ok {
		if modelPolicy.RateLimit != "" {
			result.RateLimit = modelPolicy.RateLimit
		}
		if modelPolicy.MaxTokens > 0 {
			result.MaxTokens = modelPolicy.MaxTokens
		}
		return result
	}

	// Try matching without tag (e.g., "mistral:latest" -> "mistral")
	if idx := strings.Index(modelName, ":"); idx > 0 {
		baseName := modelName[:idx]
		if modelPolicy, ok := p.Models[baseName]; ok {
			if modelPolicy.RateLimit != "" {
				result.RateLimit = modelPolicy.RateLimit
			}
			if modelPolicy.MaxTokens > 0 {
				result.MaxTokens = modelPolicy.MaxTokens
			}
			return result
		}
	}

	// Try wildcard matching (e.g., "mistral:*" matches "mistral:latest")
	for pattern, modelPolicy := range p.Models {
		if strings.HasSuffix(pattern, ":*") || strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, ":*")
			prefix = strings.TrimSuffix(prefix, "*")
			if strings.HasPrefix(modelName, prefix) {
				if modelPolicy.RateLimit != "" {
					result.RateLimit = modelPolicy.RateLimit
				}
				if modelPolicy.MaxTokens > 0 {
					result.MaxTokens = modelPolicy.MaxTokens
				}
				return result
			}
		}
	}

	return result
}

// KeyConfig represents an API key configuration
type KeyConfig struct {
	Name             string   `yaml:"name"`
	Key              string   `yaml:"key"`
	AllowedUpstreams []string `yaml:"allowed_upstreams"`
	Policy           string   `yaml:"policy"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Upstreams: make(map[string]Upstream),
		Policies: map[string]Policy{
			"default": {
				RateLimit: "60/min",
				MaxTokens: 4096,
			},
		},
		Keys: []KeyConfig{},
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Save saves configuration to a file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the default config path
func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "modeltunnel.yaml"
	}
	return filepath.Join(home, ".config", "modeltunnel", "config.yaml")
}
