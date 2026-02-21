package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/modeltunnel/modeltunnel/pkg/openai"
)

// ModelTunnelProvider implements Provider for generic OpenAI-compatible endpoints
// (including other Modeltunnel instances).
type ModelTunnelProvider struct {
	BaseProvider
}

// NewModelTunnelProvider creates a new OpenAI-compatible provider.
func NewModelTunnelProvider(id, name, apiKey string) *ModelTunnelProvider {
	baseURL := "http://127.0.0.1:8080/v1"
	return &ModelTunnelProvider{
		BaseProvider: NewBaseProvider(id, name, apiKey, baseURL),
	}
}

func (p *ModelTunnelProvider) Type() string { return "modeltunnel" }
func (p *ModelTunnelProvider) Name() string { return p.name }

func (p *ModelTunnelProvider) ListModels(ctx context.Context) ([]openai.Model, error) {
	resp, err := p.doRequest(ctx, "GET", "/models", nil)
	if err != nil {
		return nil, fmt.Errorf("list models request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list models failed: %s - %s", resp.Status, string(body))
	}

	var result openai.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Data, nil
}

func (p *ModelTunnelProvider) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	openAIReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	if req.Temperature != nil {
		openAIReq["temperature"] = req.Temperature
	}
	if req.MaxTokens != nil {
		openAIReq["max_tokens"] = req.MaxTokens
	}
	if req.TopP != nil {
		openAIReq["top_p"] = req.TopP
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

	var out openai.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &out, nil
}

func (p *ModelTunnelProvider) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error {
	// Force streaming
	req.Stream = true

	openAIReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	if req.Temperature != nil {
		openAIReq["temperature"] = req.Temperature
	}
	if req.MaxTokens != nil {
		openAIReq["max_tokens"] = req.MaxTokens
	}
	if req.TopP != nil {
		openAIReq["top_p"] = req.TopP
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

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			return nil
		}

		var chunk openai.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Ignore malformed chunk
			continue
		}
		send(chunk)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}

func (p *ModelTunnelProvider) ValidateAPIKey(ctx context.Context) error {
	_, err := p.ListModels(ctx)
	return err
}

func (p *ModelTunnelProvider) EstimateCost(model string, inputTokens, outputTokens int64) float64 {
	// Unknown pricing for generic endpoints.
	return 0
}
