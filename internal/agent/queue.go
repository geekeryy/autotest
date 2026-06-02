package agent

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryTaskQueue implements TaskQueue using in-memory storage.
// For production, this should be backed by PostgreSQL.
type InMemoryTaskQueue struct {
	jobs map[uuid.UUID]*BackgroundJob
	mu   sync.RWMutex
}

// NewInMemoryTaskQueue creates a new InMemoryTaskQueue.
func NewInMemoryTaskQueue() *InMemoryTaskQueue {
	return &InMemoryTaskQueue{
		jobs: make(map[uuid.UUID]*BackgroundJob),
	}
}

// Submit submits a task for background execution.
func (q *InMemoryTaskQueue) Submit(ctx context.Context, task SubAgentTask) (*BackgroundJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job := &BackgroundJob{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	q.jobs[job.ID] = job
	return job, nil
}

// GetStatus returns a snapshot of the background job status.
func (q *InMemoryTaskQueue) GetStatus(ctx context.Context, jobID uuid.UUID) (*BackgroundJob, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}
	snap := job.Snapshot()
	return &snap, nil
}

// Cancel cancels a running background job.
func (q *InMemoryTaskQueue) Cancel(ctx context.Context, jobID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	if job.Status == TaskStatusRunning || job.Status == TaskStatusPending {
		job.SetStatus(TaskStatusStopped)
	}
	return nil
}

// UpdateJob updates a job's status.
func (q *InMemoryTaskQueue) UpdateJob(jobID uuid.UUID, status TaskStatus, progress float64, result json.RawMessage, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	job.Status = status
	job.Progress = progress
	job.UpdatedAt = time.Now()
	if result != nil {
		job.Result = result
	}
	if errMsg != "" {
		job.Error = errMsg
	}
}

// ListJobs returns snapshots of all jobs.
func (q *InMemoryTaskQueue) ListJobs() []*BackgroundJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	jobs := make([]*BackgroundJob, 0, len(q.jobs))
	for _, job := range q.jobs {
		snap := job.Snapshot()
		jobs = append(jobs, &snap)
	}
	return jobs
}
