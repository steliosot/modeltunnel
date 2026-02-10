package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ErrorCode string

const (
	ErrTunnelUnavailable ErrorCode = "TUNNEL_UNAVAILABLE"
	ErrRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrUpstreamDown      ErrorCode = "UPSTREAM_UNAVAILABLE"
	ErrModelNotFound     ErrorCode = "MODEL_NOT_FOUND"
	ErrGPUOutOfMemory    ErrorCode = "GPU_OOM"
	ErrInvalidAPIKey     ErrorCode = "INVALID_API_KEY"
	ErrMissingAPIKey     ErrorCode = "MISSING_API_KEY"
	ErrTimeout           ErrorCode = "REQUEST_TIMEOUT"
	ErrInternal          ErrorCode = "INTERNAL_ERROR"
	ErrValidation        ErrorCode = "VALIDATION_ERROR"
)

type APIError struct {
	Code            ErrorCode  `json:"code"`
	Message         string     `json:"message"`
	Details         string     `json:"details,omitempty"`
	Action          string     `json:"action,omitempty"`
	DocsURL         string     `json:"docs_url,omitempty"`
	RequestID       string     `json:"request_id"`
	Timestamp       time.Time  `json:"timestamp"`
	RetryAfter      int        `json:"retry_after,omitempty"`
	ResetTime       *time.Time `json:"reset_time,omitempty"`
	StudentFriendly string     `json:"student_friendly,omitempty"`
	AvailableModels []string   `json:"available_models,omitempty"`
	SimilarModels   []string   `json:"similar_models,omitempty"`
}

func (e APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func RespondWithError(w http.ResponseWriter, statusCode int, err APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err,
	})
}

func NewUpstreamError(requestID string, message, details, action string) APIError {
	return APIError{
		Code:            ErrUpstreamDown,
		Message:         message,
		Details:         details,
		Action:          action,
		DocsURL:         "https://github.com/steliosot/modeltunnel#troubleshooting",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		StudentFriendly: "The AI server is currently unavailable. Please wait a moment and try again.",
	}
}

func NewOllamaNotRunningError(requestID string) APIError {
	return NewUpstreamError(
		requestID,
		"Ollama service is not responding",
		"Connection refused on 127.0.0.1:11434",
		"Start Ollama with: ollama serve",
	)
}

func NewRateLimitError(requestID, keyName string, current, limit int, resetTime time.Time) APIError {
	retryAfter := int(time.Until(resetTime).Seconds())
	if retryAfter < 0 {
		retryAfter = 0
	}

	return APIError{
		Code:            ErrRateLimitExceeded,
		Message:         fmt.Sprintf("API key '%s' exceeded rate limit", keyName),
		Details:         fmt.Sprintf("Current count: %d/%d requests", current, limit),
		Action:          "Wait before retrying or contact admin to increase limits",
		DocsURL:         "https://github.com/steliosot/modeltunnel#rate-limiting",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		RetryAfter:      retryAfter,
		ResetTime:       &resetTime,
		StudentFriendly: "You've made too many requests. Please wait a bit before trying again.",
	}
}

func NewInvalidAPIKeyError(requestID string) APIError {
	return APIError{
		Code:            ErrInvalidAPIKey,
		Message:         "Invalid or revoked API key",
		Details:         "The provided API key is not valid",
		Action:          "Check your API key in ~/.config/modeltunnel/keys.yaml or create a new key",
		DocsURL:         "https://github.com/steliosot/modeltunnel#api-keys",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		StudentFriendly: "Your access key is not valid. Please check with your teacher for the correct key.",
	}
}

func NewMissingAPIKeyError(requestID string) APIError {
	return APIError{
		Code:            ErrMissingAPIKey,
		Message:         "API key is required",
		Details:         "No Authorization header provided",
		Action:          "Add header: Authorization: Bearer YOUR_API_KEY",
		DocsURL:         "https://github.com/steliosot/modeltunnel#authentication",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		StudentFriendly: "You need an access key to use this service. Please ask your teacher for one.",
	}
}

func NewModelNotFoundError(requestID, requestedModel string, availableModels, similarModels []string) APIError {
	return APIError{
		Code:            ErrModelNotFound,
		Message:         fmt.Sprintf("Model '%s' not available", requestedModel),
		Details:         "The requested model is not loaded in the upstream server",
		Action:          fmt.Sprintf("Pull the model with: ollama pull %s", suggestedModel(availableModels, requestedModel)),
		DocsURL:         "https://github.com/steliosot/modeltunnel#models",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		AvailableModels: availableModels,
		SimilarModels:   similarModels,
		StudentFriendly: "The AI model you requested is not available. Try one of the suggested models instead.",
	}
}

func suggestedModel(available []string, requested string) string {
	if len(available) == 0 {
		return "llama3.2"
	}
	return available[0]
}

func NewInternalError(requestID string, err error) APIError {
	return APIError{
		Code:            ErrInternal,
		Message:         "Internal server error",
		Details:         err.Error(),
		Action:          "Please try again or contact support if the problem persists",
		DocsURL:         "https://github.com/steliosot/modeltunnel/issues",
		RequestID:       requestID,
		Timestamp:       time.Now(),
		StudentFriendly: "Something went wrong on the server. Please try again or tell your teacher.",
	}
}
