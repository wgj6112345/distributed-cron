package domain

import (
	"fmt"
	"time"
)

// ExecutorType defines the type of the job executor.
type ExecutorType string

const (
	ExecutorTypeHTTP  ExecutorType = "http"
	ExecutorTypeShell ExecutorType = "shell"
)

// JobExecutor represents the action to be performed when a job triggers.
type JobExecutor struct {
	URL     string `json:"url,omitempty"`    // For HTTP executor
	Method  string `json:"method,omitempty"` // For HTTP executor
	Command string `json:"command,omitempty"`// For Shell executor
}

// RetryPolicy defines the retry strategy for a job upon failure.
type RetryPolicy struct {
	MaxRetries int           `json:"max_retries"`
	Backoff    time.Duration `json:"backoff"`
}

// ConcurrencyPolicy defines how concurrent executions of the same job are handled.
type ConcurrencyPolicy string

const (
	ConcurrencyPolicyAllow  ConcurrencyPolicy = "Allow"
	ConcurrencyPolicyForbid ConcurrencyPolicy = "Forbid"
)

// Job represents a scheduled task in the distributed cron system.
type Job struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	CronExpr          string            `json:"cron_expr"`
	ExecutorType      ExecutorType      `json:"executor_type"`
	Executor          JobExecutor       `json:"executor"`
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrency_policy,omitempty"`
	RetryPolicy       *RetryPolicy      `json:"retry_policy,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// Validate checks if the job definition is valid.
func (j *Job) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("job name cannot be empty")
	}
	if j.CronExpr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}
	switch j.ExecutorType {
	case ExecutorTypeHTTP:
		if j.Executor.URL == "" {
			return fmt.Errorf("executor URL cannot be empty for http job")
		}
		if j.Executor.Method == "" {
			j.Executor.Method = "GET"
		}
	case ExecutorTypeShell:
		if j.Executor.Command == "" {
			return fmt.Errorf("executor command cannot be empty for shell job")
		}
	default:
		return fmt.Errorf("invalid executor type: %s", j.ExecutorType)
	}

	if j.ConcurrencyPolicy == "" {
		j.ConcurrencyPolicy = ConcurrencyPolicyAllow
	}
	return nil
}