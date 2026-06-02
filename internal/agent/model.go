// Package agent implements a generalized multi-agent framework.
// It supports synchronous subtasks and asynchronous background tasks,
// extending the genagent pattern to a通用的 agent orchestration system.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusStopped   TaskStatus = "stopped"
	TaskStatusTimeout   TaskStatus = "timeout"
)

// SubAgentTask defines a sub-agent task.
type SubAgentTask struct {
	ID          uuid.UUID              `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Tools       []string               `json:"tools"`
	MaxHops     int                    `json:"maxHops"`
	Context     map[string]any         `json:"context"`
	Timeout     time.Duration          `json:"timeout"`
}

// SubAgentResult is the result of a sub-agent execution.
type SubAgentResult struct {
	TaskID   uuid.UUID `json:"taskId"`
	Status   string    `json:"status"`
	Content  string    `json:"content"`
	ToolHops int       `json:"toolHops"`
	Tokens   int       `json:"tokens"`
	Error    string    `json:"error,omitempty"`
}

// BackgroundJob represents an asynchronous background task.
// All field access must go through the getter/setter methods to ensure
// thread-safe reads and writes.
type BackgroundJob struct {
	mu        sync.RWMutex `json:"-"`
	ID        uuid.UUID    `json:"id"`
	TaskID    uuid.UUID    `json:"taskId"`
	Status    TaskStatus   `json:"status"`
	Progress  float64      `json:"progress"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// SetStatus safely updates the job status and timestamp.
func (j *BackgroundJob) SetStatus(status TaskStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	j.UpdatedAt = time.Now()
}

// SetError safely sets the error message and marks the job as failed.
func (j *BackgroundJob) SetError(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Error = err
	j.Status = TaskStatusFailed
	j.UpdatedAt = time.Now()
}

// SetResult safely sets the result and marks the job as completed.
func (j *BackgroundJob) SetResult(result json.RawMessage) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Result = result
	j.Status = TaskStatusCompleted
	j.UpdatedAt = time.Now()
}

// Snapshot returns a thread-safe copy of the job's current state (without the lock).
func (j *BackgroundJob) Snapshot() BackgroundJob {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return BackgroundJob{
		ID:        j.ID,
		TaskID:    j.TaskID,
		Status:    j.Status,
		Progress:  j.Progress,
		Result:    j.Result,
		Error:     j.Error,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

// ProgressEvent reports progress of a background task.
type ProgressEvent struct {
	JobID     string  `json:"jobId"`
	Status    string  `json:"status"`
	Progress  float64 `json:"progress"`
	Message   string  `json:"message,omitempty"`
	Phase     string  `json:"phase,omitempty"`
}

// SubAgentRunner defines the interface for executing sub-agent tasks.
type SubAgentRunner interface {
	// RunSubAgent executes a sub-agent task synchronously.
	RunSubAgent(ctx context.Context, parentCfg any, task SubAgentTask) (*SubAgentResult, error)

	// RunBackground executes a task asynchronously.
	RunBackground(ctx context.Context, task SubAgentTask, progressFn func(ProgressEvent)) (*BackgroundJob, error)
}

// TaskQueue manages background task execution.
type TaskQueue interface {
	// Submit submits a task for background execution.
	Submit(ctx context.Context, task SubAgentTask) (*BackgroundJob, error)

	// GetStatus returns the status of a background job.
	GetStatus(ctx context.Context, jobID uuid.UUID) (*BackgroundJob, error)

	// Cancel cancels a running background job.
	Cancel(ctx context.Context, jobID uuid.UUID) error
}

// Orchestrator coordinates multiple sub-agents and background tasks.
type Orchestrator struct {
	runner SubAgentRunner
	queue  TaskQueue
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(runner SubAgentRunner, queue TaskQueue) *Orchestrator {
	return &Orchestrator{
		runner: runner,
		queue:  queue,
	}
}

// RunSubTask executes a sub-task synchronously.
func (o *Orchestrator) RunSubTask(ctx context.Context, parentCfg any, task SubAgentTask) (*SubAgentResult, error) {
	if o.runner == nil {
		return nil, ErrRunnerNotConfigured
	}
	return o.runner.RunSubAgent(ctx, parentCfg, task)
}

// RunBackgroundTask submits a task for background execution.
func (o *Orchestrator) RunBackgroundTask(ctx context.Context, task SubAgentTask) (*BackgroundJob, error) {
	if o.queue == nil {
		return nil, ErrQueueNotConfigured
	}
	return o.queue.Submit(ctx, task)
}

// RunParallel executes multiple sub-tasks in parallel.
func (o *Orchestrator) RunParallel(ctx context.Context, parentCfg any, tasks []SubAgentTask) ([]*SubAgentResult, error) {
	if len(tasks) == 0 {
		return []*SubAgentResult{}, nil
	}

	results := make([]*SubAgentResult, len(tasks))
	errs := make([]error, len(tasks))

	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t SubAgentTask) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errs[idx] = fmt.Errorf("sub-agent panic: %v", r)
				}
			}()
			results[idx], errs[idx] = o.RunSubTask(ctx, parentCfg, t)
		}(i, task)
	}
	wg.Wait()

	var firstErr error
	for _, err := range errs {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return results, firstErr
}

// Errors
var (
	ErrRunnerNotConfigured = errors.New("sub-agent runner not configured")
	ErrQueueNotConfigured  = errors.New("task queue not configured")
	ErrJobNotFound         = errors.New("job not found")
)

// AgentError represents an agent error.
type AgentError struct {
	Code    string
	Message string
}

func (e *AgentError) Error() string {
	return e.Message
}
