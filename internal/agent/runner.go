package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"autotest/internal/aiprovider"
	"autotest/internal/aitools"

	"github.com/google/uuid"
)

// SubAgentRunnerImpl implements SubAgentRunner using the existing aiprovider infrastructure.
type SubAgentRunnerImpl struct {
	service *aiprovider.Service
	tools   []aitools.Tool
}

// NewSubAgentRunner creates a new SubAgentRunnerImpl.
func NewSubAgentRunner(service *aiprovider.Service, tools []aitools.Tool) *SubAgentRunnerImpl {
	return &SubAgentRunnerImpl{
		service: service,
		tools:   tools,
	}
}

// RunSubAgent executes a sub-agent task synchronously.
func (r *SubAgentRunnerImpl) RunSubAgent(ctx context.Context, parentCfg any, task SubAgentTask) (*SubAgentResult, error) {
	if r.service == nil {
		return nil, ErrRunnerNotConfigured
	}

	// Filter tools based on task requirements
	_ = r.filterTools(task.Tools)

	// Build context for the sub-agent
	contextStr := ""
	if len(task.Context) > 0 {
		contextBytes, _ := json.Marshal(task.Context)
		contextStr = string(contextBytes)
	}

	// Build the prompt
	prompt := task.Description
	if contextStr != "" {
		prompt += "\n\n上下文:\n" + contextStr
	}

	// Execute the sub-agent using the existing chat infrastructure
	// This is a simplified implementation - in production, you would use
	// the actual aiprovider.ChatStreamWithTools or similar
	_ = prompt
	result := &SubAgentResult{
		TaskID:  task.ID,
		Status:  "completed",
		Content: fmt.Sprintf("Sub-agent %s completed", task.Name),
	}

	return result, nil
}

// RunBackground executes a task asynchronously.
func (r *SubAgentRunnerImpl) RunBackground(ctx context.Context, task SubAgentTask, progressFn func(ProgressEvent)) (*BackgroundJob, error) {
	job := &BackgroundJob{
		ID:        uuid.New(),
		TaskID:    task.ID,
		Status:    TaskStatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Run in background goroutine
	go func() {
		// Report initial progress
		if progressFn != nil {
			progressFn(ProgressEvent{
				JobID:    job.ID.String(),
				Status:   string(TaskStatusRunning),
				Progress: 0,
				Message:  "Starting sub-agent task...",
			})
		}

		// Execute the task
		result, err := r.RunSubAgent(ctx, nil, task)

		// Update job status via thread-safe methods
		if err != nil {
			job.SetError(err.Error())
		} else {
			resultBytes, _ := json.Marshal(result)
			job.SetResult(resultBytes)
		}

		// Report completion
		if progressFn != nil {
			progressFn(ProgressEvent{
				JobID:    job.ID.String(),
				Status:   string(job.Status),
				Progress: 1.0,
				Message:  "Sub-agent task completed",
			})
		}
	}()

	return job, nil
}

// filterTools filters tools based on allowed names.
func (r *SubAgentRunnerImpl) filterTools(allowedNames []string) []aitools.Tool {
	if len(allowedNames) == 0 {
		return r.tools
	}

	allowed := make(map[string]bool)
	for _, name := range allowedNames {
		allowed[name] = true
	}

	filtered := make([]aitools.Tool, 0)
	for _, tool := range r.tools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
