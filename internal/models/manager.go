package models

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PullStatus represents the status of a model pull operation
type PullStatus string

const (
	PullStatusQueued    PullStatus = "queued"
	PullStatusPulling   PullStatus = "pulling"
	PullStatusCompleted PullStatus = "completed"
	PullStatusFailed    PullStatus = "failed"
)

// PullProgress tracks the progress of a model pull
type PullProgress struct {
	JobID     string     `json:"job_id"`
	ModelName string     `json:"model_name"`
	Status    PullStatus `json:"status"`
	Progress  int        `json:"progress"` // 0-100
	Message   string     `json:"message"`
	Error     string     `json:"error,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Manager handles model operations
type Manager struct {
	baseURL      string
	client       *http.Client
	pullJobs     map[string]*PullProgress
	pullJobsLock sync.RWMutex
}

// NewManager creates a new model manager
func NewManager(ollamaBaseURL string) *Manager {
	if ollamaBaseURL == "" {
		ollamaBaseURL = "http://127.0.0.1:11434"
	}
	return &Manager{
		baseURL:  ollamaBaseURL,
		client:   &http.Client{Timeout: 0}, // No timeout for pull operations
		pullJobs: make(map[string]*PullProgress),
	}
}

// PullModel starts pulling a model
func (m *Manager) PullModel(ctx context.Context, modelName string) (*PullProgress, error) {
	jobID := fmt.Sprintf("pull_%d", time.Now().UnixNano())

	progress := &PullProgress{
		JobID:     jobID,
		ModelName: modelName,
		Status:    PullStatusPulling,
		Progress:  0,
		Message:   "Starting pull...",
		UpdatedAt: time.Now(),
	}

	m.pullJobsLock.Lock()
	m.pullJobs[jobID] = progress
	m.pullJobsLock.Unlock()

	go m.executePull(ctx, jobID, modelName)

	return progress, nil
}

// executePull handles the actual pull operation
func (m *Manager) executePull(ctx context.Context, jobID, modelName string) {
	url := fmt.Sprintf("%s/api/pull", m.baseURL)
	payload := fmt.Sprintf(`{"name":"%s","stream":true}`, modelName)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		m.updatePullStatus(jobID, PullStatusFailed, 0, "", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.updatePullStatus(jobID, PullStatusFailed, 0, "", err.Error())
		return
	}
	defer resp.Body.Close()

	// Stream and parse progress
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			m.updatePullStatus(jobID, PullStatusFailed, 0, "", ctx.Err().Error())
			return
		default:
			line := scanner.Text()
			m.parsePullProgress(jobID, line)
		}
	}

	// Check for completion
	if resp.StatusCode == http.StatusOK {
		m.updatePullStatus(jobID, PullStatusCompleted, 100, "Model pull completed!", "")
	} else {
		m.updatePullStatus(jobID, PullStatusFailed, 0, "", fmt.Sprintf("Pull failed with status: %d", resp.StatusCode))
	}
}

// parsePullProgress parses a line from Ollama's streaming response
func (m *Manager) parsePullProgress(jobID, line string) {
	if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return
	}

	status, _ := data["status"].(string)
	total, _ := data["total"].(float64)
	completed, _ := data["completed"].(float64)
	download, _ := data["download"].(float64)

	var message string
	var progress int

	if total > 0 && completed > 0 {
		percent := (completed / total) * 100
		progress = int(percent)
		message = fmt.Sprintf("Downloading... %d%% (%.2fGB / %.2fGB)", progress, download/1024/1024/1024, total/1024/1024/1024)
	} else {
		message = status
	}

	m.updatePullStatus(jobID, PullStatusPulling, progress, message, "")
}

// updatePullStatus updates the progress of a pull job
func (m *Manager) updatePullStatus(jobID string, status PullStatus, progress int, message, errorMsg string) {
	m.pullJobsLock.Lock()
	defer m.pullJobsLock.Unlock()

	if job, exists := m.pullJobs[jobID]; exists {
		job.Status = status
		job.Progress = progress
		job.Message = message
		if errorMsg != "" {
			job.Error = errorMsg
		}
		job.UpdatedAt = time.Now()
	}
}

// GetPullProgress returns the current progress of a pull job
func (m *Manager) GetPullProgress(jobID string) *PullProgress {
	m.pullJobsLock.RLock()
	defer m.pullJobsLock.RUnlock()

	if job, exists := m.pullJobs[jobID]; exists {
		// Clean up old completed/failed jobs
		if job.Status == PullStatusCompleted || job.Status == PullStatusFailed {
			if time.Since(job.UpdatedAt) > 5*time.Minute {
				delete(m.pullJobs, jobID)
			}
		}
		return job
	}
	return nil
}

// RemoveModel deletes a model from the local system
func (m *Manager) RemoveModel(ctx context.Context, modelName string) error {
	url := fmt.Sprintf("%s/api/delete", m.baseURL)
	payload := fmt.Sprintf(`{"name":"%s"}`, modelName)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListInstalledModels returns a list of installed models from Ollama
func (m *Manager) ListInstalledModels(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", m.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models, ok := data["models"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	var modelNames []string
	for _, m := range models {
		if model, ok := m.(map[string]interface{}); ok {
			if name, ok := model["name"].(string); ok {
				modelNames = append(modelNames, name)
			}
		}
	}

	return modelNames, nil
}
