package domain

import "context"

type LeaderElectionManager interface {
	Campaign(ctx context.Context) (<-chan struct{}, error)
	Resign(ctx context.Context) error
	IsLeader() bool
}
