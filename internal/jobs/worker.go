package jobs

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/modeltunnel/modeltunnel/internal/upstream"
)

// Queue manages pending jobs
type Queue struct {
	jobs chan string // Channel of job IDs
}

// NewQueue creates a new job queue
func NewQueue(bufferSize int) *Queue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &Queue{
		jobs: make(chan string, bufferSize),
	}
}

// Enqueue adds a job to the queue
func (q *Queue) Enqueue(jobID string) {
	select {
	case q.jobs <- jobID:
		// Job added successfully
	default:
		// Queue is full, log and continue
		log.Printf("⚠️ Job queue full, job %s may be delayed", jobID)
		// Try again with blocking (will wait)
		q.jobs <- jobID
	}
}

// Dequeue removes and returns a job from the queue
func (q *Queue) Dequeue() (string, bool) {
	select {
	case jobID := <-q.jobs:
		return jobID, true
	default:
		return "", false
	}
}

// Worker processes jobs from the queue
type Worker struct {
	id        int
	queue     *Queue
	store     *Store
	upstreams *upstream.Manager
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewWorker creates a new worker
func NewWorker(id int, queue *Queue, store *Store, upstreams *upstream.Manager) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		id:        id,
		queue:     queue,
		store:     store,
		upstreams: upstreams,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins processing jobs
func (w *Worker) Start() {
	log.Printf("🔄 Worker %d started", w.id)

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("🛑 Worker %d stopped", w.id)
			return
		case jobID := <-w.queue.jobs:
			w.processJob(jobID)
		}
	}
}

// Stop halts the worker
func (w *Worker) Stop() {
	w.cancel()
}

// processJob handles a single job
func (w *Worker) processJob(jobID string) {
	// Get job from store
	job, ok := w.store.Get(jobID)
	if !ok {
		log.Printf("❌ Worker %d: Job %s not found", w.id, jobID)
		return
	}

	// Update status to running
	w.store.UpdateStatus(jobID, StatusRunning)
	log.Printf("▶️ Worker %d: Processing job %s", w.id, jobID)

	// Get upstream
	upstreamName := "default"
	if job.Request != nil && job.Request.Model != "" {
		// Allow specifying upstream as "upstream/model" like the main API.
		if strings.Contains(job.Request.Model, "/") {
			parts := strings.SplitN(job.Request.Model, "/", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				upstreamName = parts[0]
				job.Request.Model = parts[1]
			}
		}
	}

	up, ok := w.upstreams.Get(upstreamName)
	if !ok {
		up, ok = w.upstreams.GetDefault()
		if !ok {
			w.store.SetError(jobID, fmt.Errorf("no upstream available"))
			return
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(w.ctx, 120*time.Second)
	defer cancel()

	// Process the request
	result, err := up.ChatCompletion(ctx, job.Request)
	if err != nil {
		log.Printf("❌ Worker %d: Job %s failed: %v", w.id, jobID, err)
		w.store.SetError(jobID, err)
		return
	}

	// Store result
	w.store.SetResult(jobID, result)
	log.Printf("✅ Worker %d: Job %s completed", w.id, jobID)
}

// WorkerPool manages multiple workers
type WorkerPool struct {
	workers []*Worker
	queue   *Queue
	store   *Store
}

// NewWorkerPool creates a pool of workers
func NewWorkerPool(numWorkers int, queue *Queue, store *Store, upstreams *upstream.Manager) *WorkerPool {
	pool := &WorkerPool{
		workers: make([]*Worker, numWorkers),
		queue:   queue,
		store:   store,
	}

	for i := 0; i < numWorkers; i++ {
		pool.workers[i] = NewWorker(i+1, queue, store, upstreams)
	}

	return pool
}

// Start begins all workers
func (p *WorkerPool) Start() {
	log.Printf("🚀 Starting worker pool with %d workers", len(p.workers))
	for _, worker := range p.workers {
		go worker.Start()
	}
}

// Stop halts all workers
func (p *WorkerPool) Stop() {
	log.Printf("🛑 Stopping worker pool")
	for _, worker := range p.workers {
		worker.Stop()
	}
}
