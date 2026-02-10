package router

import (
	"strings"

	"github.com/modeltunnel/modeltunnel/internal/config"
)

// IntentRouter handles model selection based on intent
type IntentRouter struct {
	intents map[string]IntentConfig
}

// IntentConfig defines routing for an intent
type IntentConfig struct {
	Models      []string // Ordered list of preferred models
	Description string
	Temperature float32
	MaxTokens   int
}

// NewIntentRouter creates a router with default intents
func NewIntentRouter() *IntentRouter {
	return &IntentRouter{
		intents: map[string]IntentConfig{
			"plan": {
				Models:      []string{"deepseek-r1", "qwen2.5", "mistral"},
				Description: "Planning, strategy, reasoning",
				Temperature: 0.3,
				MaxTokens:   4000,
			},
			"code": {
				Models:      []string{"qwen2.5", "mistral", "phi"},
				Description: "Programming, debugging, technical",
				Temperature: 0.2,
				MaxTokens:   2000,
			},
			"chat": {
				Models:      []string{"phi", "tinyllama", "mistral"},
				Description: "General chat, Q&A, support",
				Temperature: 0.7,
				MaxTokens:   1000,
			},
		},
	}
}

// NewIntentRouterFromConfig creates a router from configuration
func NewIntentRouterFromConfig(intents map[string]config.Intent) *IntentRouter {
	if len(intents) == 0 {
		// Fall back to defaults if no config provided
		return NewIntentRouter()
	}

	router := &IntentRouter{
		intents: make(map[string]IntentConfig),
	}

	for name, intent := range intents {
		router.intents[name] = IntentConfig{
			Models:      intent.Priority,
			Description: intent.Description,
			Temperature: intent.Temperature,
			MaxTokens:   intent.MaxTokens,
		}
	}

	return router
}

// Route selects the best model based on intent and available models
func (r *IntentRouter) Route(intent string, availableModels []string) (string, float32, int) {
	// Normalize intent
	intent = strings.ToLower(strings.TrimSpace(intent))

	// Get intent config or default to "chat"
	config, exists := r.intents[intent]
	if !exists {
		config = r.intents["chat"]
	}

	// Find first available model from preferred list
	for _, preferred := range config.Models {
		for _, available := range availableModels {
			// Check if available model matches preferred (handles "model:latest" format)
			if strings.HasPrefix(available, preferred) ||
				strings.Contains(available, preferred) {
				return available, config.Temperature, config.MaxTokens
			}
		}
	}

	// Fallback to first available model
	if len(availableModels) > 0 {
		return availableModels[0], config.Temperature, config.MaxTokens
	}

	// Ultimate fallback
	return "mistral", 0.7, 1000
}

// GetIntents returns list of supported intents
func (r *IntentRouter) GetIntents() map[string]string {
	result := make(map[string]string)
	for intent, config := range r.intents {
		result[intent] = config.Description
	}
	return result
}
