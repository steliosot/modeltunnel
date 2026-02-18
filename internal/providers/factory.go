package providers

import (
	"fmt"
)

// ProviderFactory creates provider instances
type ProviderFactory struct{}

// NewProvider creates a new provider based on type
func NewProvider(providerType, id, name, apiKey, baseURL string) (Provider, error) {
	switch providerType {
	case "openai":
		p := NewOpenAIProvider(id, name, apiKey)
		if baseURL != "" {
			p.baseURL = baseURL
		}
		return p, nil

	case "anthropic":
		p := NewAnthropicProvider(id, name, apiKey)
		if baseURL != "" {
			p.baseURL = baseURL
		}
		return p, nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// ProviderFromConfig creates a provider from a database Provider config
func ProviderFromConfig(config *Provider) (Provider, error) {
	return NewProvider(config.Type, config.ID, config.Name, config.APIKey, config.BaseURL)
}

// SupportedProviders returns a list of supported provider types
func SupportedProviders() []map[string]string {
	return []map[string]string{
		{
			"type":        "openai",
			"name":        "OpenAI",
			"description": "GPT-4, GPT-3.5, and other OpenAI models",
			"default_url": "https://api.openai.com/v1",
		},
		{
			"type":        "anthropic",
			"name":        "Anthropic",
			"description": "Claude 3 and Claude 2 models",
			"default_url": "https://api.anthropic.com/v1",
		},
	}
}

// DefaultModels returns the default models for a provider type
func DefaultModels(providerType string) []string {
	switch providerType {
	case "openai":
		return []string{
			"gpt-4",
			"gpt-4-turbo",
			"gpt-3.5-turbo",
		}
	case "anthropic":
		return []string{
			"claude-3-opus-20240229",
			"claude-3-sonnet-20240229",
			"claude-3-haiku-20240307",
		}
	default:
		return []string{}
	}
}
