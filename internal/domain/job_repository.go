package domain

import (
	"context"
	"errors" // Import the errors package
)

// ErrJobNotFound is a sentinel error returned when a job is not found.
var ErrJobNotFound = errors.New("job not found")

// JobRepository defines the interface for persisting and retrieving Job definitions.

type JobRepository interface {
	Save(ctx context.Context, job *Job) error
	Delete(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (*Job, error)
	List(ctx context.Context) ([]*Job, error)
}
