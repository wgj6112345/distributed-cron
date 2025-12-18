// internal/domain/execution.go
package domain

import (
	"context"
	"fmt"
	"time"
)

// ExecutionStatus defines the status of a job execution.
type ExecutionStatus string

const (
	ExecutionStatusRunning ExecutionStatus = "running"
	ExecutionStatusSuccess ExecutionStatus = "success"
	ExecutionStatusFailed  ExecutionStatus = "failed"
)

// ExecutionRecord represents a single execution instance of a job.
type ExecutionRecord struct {
	ID              string          `json:"id"`                // Unique ID for this specific execution attempt
	JobName         string          `json:"job_name"`          // Name of the job being executed
	StartTime       time.Time       `json:"start_time"`        // When the execution started
	EndTime         time.Time       `json:"end_time"`          // When the execution ended
	Status          ExecutionStatus `json:"status"`            // Status: running, success, failed
	Output          string          `json:"output,omitempty"`  // Standard output (e.g., for shell commands)
	Error           string          `json:"error,omitempty"`   // Error message if execution failed
	RetriesAttempted int            `json:"retries_attempted"` // Number of retries attempted for this execution instance
	WorkerID        string          `json:"worker_id,omitempty"` // ID of the worker that executed the job
}

// Validate checks if the execution record is valid.
func (r *ExecutionRecord) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("execution record ID cannot be empty")
	}
	if r.JobName == "" {
		return fmt.Errorf("execution record job name cannot be empty")
	}
	if r.StartTime.IsZero() {
		return fmt.Errorf("execution record start time cannot be zero")
	}
	if r.Status == "" {
		return fmt.Errorf("execution record status cannot be empty")
	}
	return nil
}


// ExecutionRepository defines the interface for persisting and retrieving execution records.
type ExecutionRepository interface {
	// Save persists a single execution record.
	Save(ctx context.Context, record *ExecutionRecord) error
	// ListByJobName retrieves historical execution records for a specific job, with pagination.
	// Records should typically be returned in reverse chronological order (newest first).
	ListByJobName(ctx context.Context, jobName string, page, pageSize int) ([]*ExecutionRecord, error)
	// Get retrieves a single execution record by its JobName and ExecutionID.
	Get(ctx context.Context, jobName, executionID string) (*ExecutionRecord, error)
}
