package upstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// OllamaError represents different types of Ollama connection errors
type OllamaError struct {
	Type        string
	Message     string
	Action      string
	IsRetryable bool
}

func (e OllamaError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// IsOllamaNotRunning checks if error indicates Ollama is not running
func IsOllamaNotRunning(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "No connection could be made") ||
		strings.Contains(errStr, "dial tcp") && strings.Contains(errStr, "connect: connection refused") ||
		errors.Is(err, syscall.ECONNREFUSED)
}

// IsOllamaTimeout checks if error indicates a timeout
func IsOllamaTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "Client.Timeout") ||
		strings.Contains(errStr, "i/o timeout")
}

// IsModelNotFound checks if error indicates model doesn't exist
func IsModelNotFound(err error, body string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(body, "model not found") ||
		strings.Contains(body, "model does not exist") ||
		strings.Contains(strings.ToLower(body), "pull the model")
}

// IsGPUOutOfMemory checks if error indicates GPU OOM
func IsGPUOutOfMemory(body string) bool {
	bodyLower := strings.ToLower(body)
	return strings.Contains(bodyLower, "out of memory") ||
		strings.Contains(bodyLower, "cuda out of memory") ||
		strings.Contains(bodyLower, "gpu memory") ||
		strings.Contains(bodyLower, "failed to allocate")
}

// ParseOllamaError converts an error into a structured OllamaError
func ParseOllamaError(err error, statusCode int, body string, baseURL string) OllamaError {
	if IsOllamaNotRunning(err) {
		return OllamaError{
			Type:        "OLLAMA_NOT_RUNNING",
			Message:     fmt.Sprintf("Ollama is not responding on %s", baseURL),
			Action:      "Start Ollama with: ollama serve",
			IsRetryable: true,
		}
	}

	if IsOllamaTimeout(err) {
		return OllamaError{
			Type:        "OLLAMA_TIMEOUT",
			Message:     "Ollama request timed out (model taking too long)",
			Action:      "Try a smaller model, reduce max_tokens, or check GPU resources",
			IsRetryable: true,
		}
	}

	if IsGPUOutOfMemory(body) {
		return OllamaError{
			Type:        "GPU_OUT_OF_MEMORY",
			Message:     "GPU ran out of memory while processing request",
			Action:      "Use a smaller model (e.g., llama3.2 instead of llama3.1) or reduce batch size",
			IsRetryable: false,
		}
	}

	if IsModelNotFound(err, body) {
		return OllamaError{
			Type:        "MODEL_NOT_FOUND",
			Message:     "The requested model is not available in Ollama",
			Action:      "Pull the model first: ollama pull <model-name>",
			IsRetryable: false,
		}
	}

	if statusCode == http.StatusNotFound {
		return OllamaError{
			Type:        "ENDPOINT_NOT_FOUND",
			Message:     "Ollama API endpoint not found",
			Action:      "Check if Ollama is running and accessible",
			IsRetryable: true,
		}
	}

	if statusCode >= 500 {
		return OllamaError{
			Type:        "OLLAMA_SERVER_ERROR",
			Message:     fmt.Sprintf("Ollama server error (status %d): %s", statusCode, body),
			Action:      "Check Ollama logs with: ollama logs",
			IsRetryable: true,
		}
	}

	// Default error
	return OllamaError{
		Type:        "OLLAMA_ERROR",
		Message:     fmt.Sprintf("Ollama error: %v (status %d)", err, statusCode),
		Action:      "Check Ollama status and try again",
		IsRetryable: true,
	}
}

// CheckOllamaHealth attempts to connect to Ollama and returns status
func CheckOllamaHealth(baseURL string) (bool, []string, error) {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// Parse available models
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true, nil, nil // Connected but couldn't parse models
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return true, models, nil
}

// SuggestModel returns similar model suggestions
func SuggestModel(requested string, available []string) []string {
	if len(available) == 0 {
		return nil
	}

	requestedLower := strings.ToLower(requested)
	var suggestions []string

	// Try to find models with similar names
	for _, model := range available {
		modelLower := strings.ToLower(model)

		// Check for common patterns
		if strings.Contains(modelLower, requestedLower) ||
			strings.Contains(requestedLower, modelLower) {
			suggestions = append(suggestions, model)
			continue
		}

		// Check for family matches (llama, mistral, qwen, etc.)
		families := []string{"llama", "mistral", "qwen", "phi", "gemma", "deepseek"}
		for _, family := range families {
			if strings.Contains(requestedLower, family) && strings.Contains(modelLower, family) {
				suggestions = append(suggestions, model)
				break
			}
		}
	}

	// If no suggestions found, return first 3 available models
	if len(suggestions) == 0 && len(available) > 0 {
		max := 3
		if len(available) < max {
			max = len(available)
		}
		suggestions = available[:max]
	}

	return suggestions
}
