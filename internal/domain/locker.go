// internal/domain/locker.go
package domain

import (
	"context"
	"errors"
)

// ErrLockNotAcquired is returned when a lock cannot be acquired, for example,
// if it's already held by another process.
var ErrLockNotAcquired = errors.New("lock not acquired")

// Lock represents an acquired distributed lock.
type Lock interface {
	// Unlock releases the lock.
	Unlock(ctx context.Context) error
}

// Locker defines the interface for a distributed locking mechanism.
type Locker interface {
	// Lock attempts to acquire a lock for the given name.
	// It should be a non-blocking call. If the lock is already held,
	// it must return ErrLockNotAcquired.
	Lock(ctx context.Context, name string) (Lock, error)
}
