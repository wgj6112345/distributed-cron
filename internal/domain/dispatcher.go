// internal/domain/dispatcher.go
package domain

import "context"

// Dispatcher defines the interface for dispatching jobs to workers.
type Dispatcher interface {
	DispatchTask(ctx context.Context, job *Job) error
}
