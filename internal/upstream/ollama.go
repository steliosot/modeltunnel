package upstream

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

// OllamaUpstream represents an Ollama upstream provider
type OllamaUpstream struct {
	baseURL string
	model   string
	client  *http.Client
}

// OllamaMessage represents a message in Ollama format
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaRequest represents an Ollama chat request
type OllamaRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// OllamaResponse represents an Ollama chat response
type OllamaResponse struct {
	Model     string        `json:"model"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
	CreatedAt string        `json:"created_at"`
}

// NewOllamaUpstream creates a new Ollama upstream
func NewOllamaUpstream(baseURL, model string) *OllamaUpstream {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &OllamaUpstream{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ListModels returns available models
func (o *OllamaUpstream) ListModels(ctx context.Context) ([]openai.Model, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		ollamaErr := ParseOllamaError(err, 0, "", o.baseURL)
		return nil, fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		ollamaErr := ParseOllamaError(nil, resp.StatusCode, string(body), o.baseURL)
		return nil, fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]openai.Model, len(result.Models))
	now := time.Now().Unix()
	for i, m := range result.Models {
		models[i] = openai.Model{
			ID:         m.Name,
			Object:     "model",
			Created:    now,
			OwnedBy:    "ollama",
			Size:       m.Size,
			ModifiedAt: m.ModifiedAt,
		}
	}

	return models, nil
}

// ChatCompletion performs a chat completion
func (o *OllamaUpstream) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = o.model
	}

	ollamaReq := OllamaRequest{
		Model:    model,
		Messages: convertMessages(req.Messages),
		Stream:   false,
		Options:  make(map[string]interface{}),
	}

	if req.Temperature != nil {
		ollamaReq.Options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		ollamaReq.Options["num_predict"] = *req.MaxTokens
	}
	if req.TopP != nil {
		ollamaReq.Options["top_p"] = *req.TopP
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		ollamaErr := ParseOllamaError(err, 0, "", o.baseURL)
		return nil, fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		ollamaErr := ParseOllamaError(nil, resp.StatusCode, string(body), o.baseURL)
		return nil, fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	return &openai.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openai.Choice{
			{
				Index: 0,
				Message: openai.Message{
					Role:    ollamaResp.Message.Role,
					Content: ollamaResp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: openai.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}, nil
}

// ChatCompletionStream performs a streaming chat completion
func (o *OllamaUpstream) ChatCompletionStream(ctx context.Context, req *openai.ChatCompletionRequest, send func(openai.ChatCompletionStreamResponse)) error {
	model := req.Model
	if model == "" {
		model = o.model
	}

	ollamaReq := OllamaRequest{
		Model:    model,
		Messages: convertMessages(req.Messages),
		Stream:   true,
		Options:  make(map[string]interface{}),
	}

	if req.Temperature != nil {
		ollamaReq.Options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		ollamaReq.Options["num_predict"] = *req.MaxTokens
	}
	if req.TopP != nil {
		ollamaReq.Options["top_p"] = *req.TopP
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		ollamaErr := ParseOllamaError(err, 0, "", o.baseURL)
		return fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		ollamaErr := ParseOllamaError(nil, resp.StatusCode, string(body), o.baseURL)
		return fmt.Errorf("%s\nAction: %s", ollamaErr.Message, ollamaErr.Action)
	}

	// Reset the body for streaming
	resp.Body = io.NopCloser(bytes.NewReader(body))
	decoder := json.NewDecoder(resp.Body)
	id := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())
	created := time.Now().Unix()

	for {
		var ollamaResp OllamaResponse
		if err := decoder.Decode(&ollamaResp); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		send(openai.ChatCompletionStreamResponse{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   model,
			Choices: []openai.StreamChoice{
				{
					Index: 0,
					Delta: openai.Delta{
						Content: ollamaResp.Message.Content,
					},
					FinishReason: func() string {
						if ollamaResp.Done {
							return "stop"
						}
						return ""
					}(),
				},
			},
		})

		if ollamaResp.Done {
			break
		}
	}

	return nil
}

func convertMessages(msgs []openai.Message) []OllamaMessage {
	result := make([]OllamaMessage, len(msgs))
	for i, m := range msgs {
		result[i] = OllamaMessage{
			Role:    m.Role,
			Content: convertContentToString(m.Content),
		}
	}
	return result
}

// convertContentToString converts message content to string for Ollama
// Handles both simple string content and OpenAI content arrays (text/image parts)
func convertContentToString(content interface{}) string {
	if content == nil {
		return ""
	}

	// If it's already a string, return as-is
	if str, ok := content.(string); ok {
		return str
	}

	// If it's an array (OpenAI content parts), extract text
	// Example: [{"type": "text", "text": "Hello"}, {"type": "image_url", ...}]
	if arr, ok := content.([]interface{}); ok {
		var text strings.Builder
		for _, part := range arr {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partType, ok := partMap["type"].(string); ok && partType == "text" {
					if textContent, ok := partMap["text"].(string); ok {
						text.WriteString(textContent)
					}
				}
			}
		}
		return text.String()
	}

	return fmt.Sprintf("%v", content)
}

// Provider returns the provider name
func (o *OllamaUpstream) Provider() string {
	return "ollama"
}
