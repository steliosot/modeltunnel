package upstream

import (
	"time"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/modeltunnel/modeltunnel/pkg/openai"
)

// VLLMUpstream represents a vLLM upstream provider
// vLLM provides OpenAI-compatible server with 14-24x higher throughput
type VLLMUpstream struct {
	baseURL string
	client  *http.Client
}

// NewVLLMUpstream creates a new vLLM upstream
func NewVLLMUpstream(baseURL string) *VLLMUpstream {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &VLLMUpstream{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ListModels returns available models from vLLM
func (v *VLLMUpstream) ListModels(ctx context.Context) ([]openai.Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", v.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vLLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vLLM returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result openai.ModelList
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse vLLM models: %w", err)
	}

	if len(result.Data) == 0 {
		return []openai.Model{}, nil
	}

	now := int64(0) // vLLM doesn't provide creation time
	for i := range result.Data {
		result.Data[i].Created = now
		result.Data[i].OwnedBy = "vllm"
	}

	return result.Data, nil
}

// ChatCompletion performs chat completion via vLLM's OpenAI API
func (v *VLLMUpstream) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	model := req.Model
	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	vllmReq := map[string]interface{}{
		"model":    model,
		"messages": req.Messages,
	}

	if req.Temperature != nil {
		vllmReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		vllmReq["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		vllmReq["top_p"] = *req.TopP
	}

	body, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", v.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vLLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vLLM returned status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read vLLM response: %w", err)
	}

	var vllmResp openai.ChatCompletionResponse
	if err := json.Unmarshal(responseBody, &vllmResp); err != nil {
		return nil, fmt.Errorf("failed to parse vLLM response: %w", err)
	}

	return &vllmResp, nil
}

// ChatCompletionStream sends streaming request via vLLM (TODO: not yet implemented)
func (v *VLLMUpstream) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error {
	return fmt.Errorf("streaming not yet implemented for vLLM - use non-streaming requests")
}

// Provider returns the provider name
func (v *VLLMUpstream) Provider() string {
	return "vllm"
}
