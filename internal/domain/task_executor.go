package domain

import "context"

// TaskExecutor defines the interface for executing a job's action.
type TaskExecutor interface {
	Execute(ctx context.Context, job *Job) (output string, err error)
}
