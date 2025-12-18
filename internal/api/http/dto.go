package http

import (
	"distributed-cron/internal/domain"
	"time"
)

// ExecutorRequest is the DTO for executor configuration.
type ExecutorRequest struct {
	URL     string `json:"url"`
	Method  string `json:"method"`
	Command string `json:"command"`
}

// RetryPolicyRequest is the DTO for retry policy configuration.
type RetryPolicyRequest struct {
	MaxRetries int    `json:"max_retries" validate:"gte=0,lte=10"`
	Backoff    string `json:"backoff" validate:"required_with=MaxRetries,duration"`
}

// SaveJobRequest is the Data Transfer Object for creating/updating a job.
type SaveJobRequest struct {
	Name              string              `json:"name" validate:"required,min=1,max=128"`
	CronExpr          string              `json:"cron_expr" validate:"required,cron"`
	ExecutorType      string              `json:"executor_type" validate:"required,oneof=http shell"`
	Executor          ExecutorRequest     `json:"executor" validate:"required"`
	ConcurrencyPolicy string              `json:"concurrency_policy" validate:"omitempty,oneof=Allow Forbid"`
	RetryPolicy       *RetryPolicyRequest `json:"retry_policy,omitempty" validate:"omitempty,dive"`
}

// ToDomainJob converts a SaveJobRequest DTO to a domain.Job object.
func (r *SaveJobRequest) ToDomainJob() *domain.Job {
	concurrencyPolicy := domain.ConcurrencyPolicy(r.ConcurrencyPolicy)
	if concurrencyPolicy == "" {
		concurrencyPolicy = domain.ConcurrencyPolicyAllow
	}

	var retryPolicy *domain.RetryPolicy
	if r.RetryPolicy != nil {
		backoff, _ := time.ParseDuration(r.RetryPolicy.Backoff)
		retryPolicy = &domain.RetryPolicy{
			MaxRetries: r.RetryPolicy.MaxRetries,
			Backoff:    backoff,
		}
	}

	// Normalize executor based on type
	executor := domain.JobExecutor{}
	executorType := domain.ExecutorType(r.ExecutorType)
	switch executorType {
	case domain.ExecutorTypeHTTP:
		executor.URL = r.Executor.URL
		executor.Method = r.Executor.Method
		if executor.Method == "" {
			executor.Method = "GET"
		}
	case domain.ExecutorTypeShell:
		executor.Command = r.Executor.Command
	}


	return &domain.Job{
		Name:              r.Name,
		CronExpr:          r.CronExpr,
		ExecutorType:      executorType,
		Executor:          executor,
		ConcurrencyPolicy: concurrencyPolicy,
		RetryPolicy:       retryPolicy,
	}
}