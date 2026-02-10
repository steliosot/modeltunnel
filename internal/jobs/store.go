package jobs

import (
	"fmt"
	"sync"
	"time"

	"github.com/modeltunnel/modeltunnel/pkg/openai"
)

// Status represents the current state of a job
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Job represents an async model request
type Job struct {
	ID          string                         `json:"job_id"`
	Status      Status                         `json:"status"`
	Request     *openai.ChatCompletionRequest  `json:"request"`
	Result      *openai.ChatCompletionResponse `json:"result,omitempty"`
	Error       string                         `json:"error,omitempty"`
	CreatedAt   time.Time                      `json:"created_at"`
	StartedAt   *time.Time                     `json:"started_at,omitempty"`
	CompletedAt *time.Time                     `json:"completed_at,omitempty"`
}

// Store manages jobs in memory
type Store struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewStore creates a new job store
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*Job),
	}
}

// Create creates a new job
func (s *Store) Create(request *openai.ChatCompletionRequest) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := &Job{
		ID:        generateJobID(),
		Status:    StatusQueued,
		Request:   request,
		CreatedAt: time.Now(),
	}

	s.jobs[job.ID] = job
	return job
}

// Get retrieves a job by ID
func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	return job, ok
}

// UpdateStatus updates the status of a job
func (s *Store) UpdateStatus(id string, status Status) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Status = status
		now := time.Now()

		switch status {
		case StatusRunning:
			job.StartedAt = &now
		case StatusCompleted, StatusFailed:
			job.CompletedAt = &now
		}
	}
}

// SetResult sets the result of a completed job
func (s *Store) SetResult(id string, result *openai.ChatCompletionResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Result = result
		job.Status = StatusCompleted
		now := time.Now()
		job.CompletedAt = &now
	}
}

// SetError sets the error of a failed job
func (s *Store) SetError(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Error = err.Error()
		job.Status = StatusFailed
		now := time.Now()
		job.CompletedAt = &now
	}
}

// List returns all jobs (for debugging/admin)
func (s *Store) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// generateJobID creates a simple job ID
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}
