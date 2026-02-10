package openai

// Model represents an OpenAI-compatible model
type Model struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	Created    int64  `json:"created"`
	OwnedBy    string `json:"owned_by"`
	Size       int64  `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
	InConfig   bool   `json:"in_config,omitempty"`  // Whether model is defined in config
	RateLimit  string `json:"rate_limit,omitempty"` // Rate limit from config (e.g., "60/min")
}

// ModelList represents a list of models
type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	Temperature      *float32  `json:"temperature,omitempty"`
	TopP             *float32  `json:"top_p,omitempty"`
	N                *int      `json:"n,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	PresencePenalty  *float32  `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32  `json:"frequency_penalty,omitempty"`
	User             string    `json:"user,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or array of content parts (text/image)
	Name    string      `json:"name,omitempty"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionStreamResponse represents a streaming response chunk
type ChatCompletionStreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a streaming choice
type StreamChoice struct {
	Index        int    `json:"index"`
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// Delta represents a delta in streaming response
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError represents an API error
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}
