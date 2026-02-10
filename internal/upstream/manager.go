package upstream

import (
	"context"

	"github.com/modeltunnel/modeltunnel/pkg/openai"
)

// Upstream defines the interface for model providers
type Upstream interface {
	ListModels(ctx context.Context) ([]openai.Model, error)
	ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error
	Provider() string
}

// Manager manages multiple upstream providers
type Manager struct {
	upstreams       map[string]Upstream
	defaultUpstream string
}

// NewManager creates a new upstream manager
func NewManager() *Manager {
	return &Manager{
		upstreams: make(map[string]Upstream),
	}
}

// Register registers an upstream
func (m *Manager) Register(name string, upstream Upstream) {
	m.upstreams[name] = upstream
	if m.defaultUpstream == "" {
		m.defaultUpstream = name
	}
}

// Get retrieves an upstream by name
func (m *Manager) Get(name string) (Upstream, bool) {
	u, ok := m.upstreams[name]
	return u, ok
}

// GetDefault returns the default upstream
func (m *Manager) GetDefault() (Upstream, bool) {
	if m.defaultUpstream == "" {
		return nil, false
	}
	return m.upstreams[m.defaultUpstream], true
}

// List returns all registered upstream names
func (m *Manager) List() []string {
	names := make([]string, 0, len(m.upstreams))
	for name := range m.upstreams {
		names = append(names, name)
	}
	return names
}

// AllModels returns models from all upstreams
func (m *Manager) AllModels(ctx context.Context) ([]openai.Model, error) {
	var allModels []openai.Model
	seen := make(map[string]bool)

	for name, upstream := range m.upstreams {
		models, err := upstream.ListModels(ctx)
		if err != nil {
			continue
		}
		for _, model := range models {
			if !seen[model.ID] {
				seen[model.ID] = true
				model.ID = name + "/" + model.ID
				allModels = append(allModels, model)
			}
		}
	}

	return allModels, nil
}
