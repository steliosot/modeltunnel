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

// AnthropicProvider implements Provider for Anthropic Claude API
type AnthropicProvider struct {
	BaseProvider
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(id, name, apiKey string) *AnthropicProvider {
	baseURL := "https://api.anthropic.com/v1"
	return &AnthropicProvider{
		BaseProvider: NewBaseProvider(id, name, apiKey, baseURL),
	}
}

// Type returns the provider type
func (p *AnthropicProvider) Type() string {
	return "anthropic"
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return p.name
}

// ListModels returns available Anthropic models
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]openai.Model, error) {
	// Anthropic doesn't have a list models endpoint, so we return known models
	models := []openai.Model{
		{ID: "claude-3-opus-20240229", Object: "model", OwnedBy: "anthropic"},
		{ID: "claude-3-sonnet-20240229", Object: "model", OwnedBy: "anthropic"},
		{ID: "claude-3-haiku-20240307", Object: "model", OwnedBy: "anthropic"},
		{ID: "claude-2.1", Object: "model", OwnedBy: "anthropic"},
		{ID: "claude-2.0", Object: "model", OwnedBy: "anthropic"},
		{ID: "claude-instant-1.2", Object: "model", OwnedBy: "anthropic"},
	}
	return models, nil
}

// convertToAnthropicMessages converts OpenAI message format to Anthropic format
func convertToAnthropicMessages(messages []openai.Message) (string, []map[string]string, error) {
	var system string
	var anthropicMessages []map[string]string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			system = msg.Content
		case "user", "assistant":
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    msg.Role,
				"content": msg.Content,
			})
		default:
			// Map other roles to user
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    "user",
				"content": msg.Content,
			})
		}
	}

	return system, anthropicMessages, nil
}

// ChatCompletion performs a chat completion
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	// Convert messages to Anthropic format
	system, messages, err := convertToAnthropicMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("convert messages: %w", err)
	}

	// Build Anthropic request
	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
	}

	if system != "" {
		anthropicReq["system"] = system
	}
	if req.Temperature != 0 {
		anthropicReq["temperature"] = req.Temperature
	}

	// Anthropic uses x-api-key header instead of Authorization
	jsonBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat completion failed: %s - %s", resp.Status, string(body))
	}

	var anthropicResp struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Role    string `json:"role"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract text content
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// Convert to OpenAI format
	response := &openai.ChatCompletionResponse{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   anthropicResp.Model,
		Choices: []openai.Choice{
			{
				Index: 0,
				Message: openai.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: openai.Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}

	return response, nil
}

// ChatCompletionStream performs a streaming chat completion
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error {
	// Convert messages to Anthropic format
	system, messages, err := convertToAnthropicMessages(req.Messages)
	if err != nil {
		return fmt.Errorf("convert messages: %w", err)
	}

	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
		"stream":     true,
	}

	if system != "" {
		anthropicReq["system"] = system
	}
	if req.Temperature != 0 {
		anthropicReq["temperature"] = req.Temperature
	}

	jsonBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stream failed: %s - %s", resp.Status, string(body))
	}

	// Read SSE stream
	decoder := json.NewDecoder(resp.Body)
	for {
		line, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read stream: %w", err)
		}

		// Skip non-string tokens (like newlines)
		if _, ok := line.(string); !ok {
			continue
		}

		lineStr := line.(string)
		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")
		if data == "[DONE]" {
			break
		}

		var event struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue // Skip malformed events
		}

		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			streamResp := openai.ChatCompletionStreamResponse{
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []openai.StreamChoice{
					{
						Index: event.Index,
						Delta: openai.Message{
							Role:    "assistant",
							Content: event.Delta.Text,
						},
					},
				},
			}
			send(streamResp)
		}
	}

	return nil
}

// ValidateAPIKey validates the API key
func (p *AnthropicProvider) ValidateAPIKey(ctx context.Context) error {
	// Make a simple request to validate the key
	req := map[string]interface{}{
		"model":      "claude-3-haiku-20240307",
		"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
		"max_tokens": 1,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}

	return nil
}

// EstimateCost estimates the cost for a request
func (p *AnthropicProvider) EstimateCost(model string, inputTokens, outputTokens int64) float64 {
	// Anthropic pricing (as of 2024) - per 1K tokens
	pricing := map[string]struct {
		Input  float64
		Output float64
	}{
		"claude-3-opus":   {Input: 0.015, Output: 0.075},
		"claude-3-sonnet": {Input: 0.003, Output: 0.015},
		"claude-3-haiku":  {Input: 0.00025, Output: 0.00125},
		"claude-2.1":      {Input: 0.008, Output: 0.024},
		"claude-2.0":      {Input: 0.008, Output: 0.024},
		"claude-instant":  {Input: 0.0008, Output: 0.0024},
	}

	// Check prefix match
	for modelPrefix, price := range pricing {
		if strings.HasPrefix(model, modelPrefix) {
			inputCost := (float64(inputTokens) / 1000) * price.Input
			outputCost := (float64(outputTokens) / 1000) * price.Output
			return inputCost + outputCost
		}
	}

	// Default pricing
	return 0
}
