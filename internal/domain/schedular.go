package domain

import "context"

type Schedular interface {
	Start(ctx context.Context) error
	Stop()

	AddJob(job *Job) error
	RemoveJob(name string) error
}
