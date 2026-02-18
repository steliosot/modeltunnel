package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/modeltunnel/modeltunnel/pkg/openai"
)

// Provider defines the interface for external API providers
type Provider interface {
	Type() string
	Name() string
	ListModels(ctx context.Context) ([]openai.Model, error)
	ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error)
	ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error
	ValidateAPIKey(ctx context.Context) error
	EstimateCost(model string, inputTokens, outputTokens int64) float64
}

// BaseProvider contains common provider functionality
type BaseProvider struct {
	id      string
	name    string
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewBaseProvider creates a base provider
func NewBaseProvider(id, name, apiKey, baseURL string) BaseProvider {
	return BaseProvider{
		id:      id,
		name:    name,
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with authentication
func (p *BaseProvider) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := fmt.Sprintf("%s%s", p.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	req.Header.Set("Content-Type", "application/json")

	return p.client.Do(req)
}

// OpenAIProvider implements Provider for OpenAI API
type OpenAIProvider struct {
	BaseProvider
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(id, name, apiKey string) *OpenAIProvider {
	baseURL := "https://api.openai.com/v1"
	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(id, name, apiKey, baseURL),
	}
}

// Type returns the provider type
func (p *OpenAIProvider) Type() string {
	return "openai"
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return p.name
}

// ListModels returns available OpenAI models
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]openai.Model, error) {
	resp, err := p.doRequest(ctx, "GET", "/models", nil)
	if err != nil {
		return nil, fmt.Errorf("list models request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list models failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Data []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var models []openai.Model
	for _, m := range result.Data {
		// Only include chat completion models
		if strings.HasPrefix(m.ID, "gpt-") || strings.HasPrefix(m.ID, "text-") {
			models = append(models, openai.Model{
				ID:      m.ID,
				Object:  "model",
				OwnedBy: "openai",
			})
		}
	}

	return models, nil
}

// ChatCompletion performs a chat completion
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	// Map our request to OpenAI format
	openAIReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	if req.Temperature != 0 {
		openAIReq["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	}
	if req.Stream {
		openAIReq["stream"] = req.Stream
	}

	resp, err := p.doRequest(ctx, "POST", "/chat/completions", openAIReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion failed: %s - %s", resp.Status, string(body))
	}

	var openAIResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Convert to our response format
	response := &openai.ChatCompletionResponse{
		ID:      openAIResp.ID,
		Object:  "chat.completion",
		Created: openAIResp.Created,
		Model:   openAIResp.Model,
		Usage: openai.Usage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}

	for _, c := range openAIResp.Choices {
		response.Choices = append(response.Choices, openai.Choice{
			Index: c.Index,
			Message: openai.Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: c.FinishReason,
		})
	}

	return response, nil
}

// ChatCompletionStream performs a streaming chat completion
func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error {
	// Force streaming
	req.Stream = true

	openAIReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	if req.Temperature != 0 {
		openAIReq["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		openAIReq["max_tokens"] = req.MaxTokens
	}

	resp, err := p.doRequest(ctx, "POST", "/chat/completions", openAIReq)
	if err != nil {
		return fmt.Errorf("stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream failed: %s - %s", resp.Status, string(body))
	}

	// Read streaming response
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode chunk: %w", err)
		}

		streamResp := openai.ChatCompletionStreamResponse{
			ID:      chunk.ID,
			Object:  chunk.Object,
			Created: chunk.Created,
			Model:   chunk.Model,
		}

		for _, c := range chunk.Choices {
			streamResp.Choices = append(streamResp.Choices, openai.StreamChoice{
				Index: c.Index,
				Delta: openai.Message{
					Role:    c.Delta.Role,
					Content: c.Delta.Content,
				},
				FinishReason: "",
			})
			if c.FinishReason != nil {
				streamResp.Choices[len(streamResp.Choices)-1].FinishReason = *c.FinishReason
			}
		}

		send(streamResp)
	}

	return nil
}

// ValidateAPIKey validates the API key by listing models
func (p *OpenAIProvider) ValidateAPIKey(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

// EstimateCost estimates the cost for a request
func (p *OpenAIProvider) EstimateCost(model string, inputTokens, outputTokens int64) float64 {
	// OpenAI pricing (as of 2024) - per 1K tokens
	pricing := map[string]struct {
		Input  float64
		Output float64
	}{
		"gpt-4":             {Input: 0.03, Output: 0.06},
		"gpt-4-turbo":       {Input: 0.01, Output: 0.03},
		"gpt-3.5-turbo":     {Input: 0.0005, Output: 0.0015},
		"gpt-3.5-turbo-16k": {Input: 0.003, Output: 0.004},
	}

	// Check exact match first
	if price, ok := pricing[model]; ok {
		inputCost := (float64(inputTokens) / 1000) * price.Input
		outputCost := (float64(outputTokens) / 1000) * price.Output
		return inputCost + outputCost
	}

	// Check prefix match
	for modelPrefix, price := range pricing {
		if strings.HasPrefix(model, modelPrefix) {
			inputCost := (float64(inputTokens) / 1000) * price.Input
			outputCost := (float64(outputTokens) / 1000) * price.Output
			return inputCost + outputCost
		}
	}

	// Default pricing for unknown models
	return 0
}
