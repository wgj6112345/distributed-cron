// internal/worker/registry.go
package worker

import (
	"context"
	"fmt"
	"log/slog"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// WorkerRegistryPrefix defines the etcd prefix where workers register themselves.
	WorkerRegistryPrefix = "/cron/workers/"
)

// Registry handles the registration of a worker in etcd.
type Registry struct {
	client  *clientv3.Client
	logger  *slog.Logger
	leaseID clientv3.LeaseID
	key     string
	value   string
}

// NewRegistry creates a new worker registry.
func NewRegistry(client *clientv3.Client, logger *slog.Logger) *Registry {
	return &Registry{
		client: client,
		logger: logger,
	}
}

// Register registers the worker with etcd, providing its address.
// It starts a keep-alive goroutine for the lease.
func (r *Registry) Register(ctx context.Context, workerID, workerAddr string, ttl int64) error {
	r.key = WorkerRegistryPrefix + workerID
	r.value = workerAddr

	// 1. Create a new lease with a TTL.
	leaseResp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}
	r.leaseID = leaseResp.ID

	// 2. Put the worker's key-value pair into etcd with the lease.
	_, err = r.client.Put(ctx, r.key, r.value, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("failed to put worker registration key: %w", err)
	}

	// 3. Start a keep-alive goroutine to periodically refresh the lease.
	keepAliveCh, err := r.client.KeepAlive(context.Background(), r.leaseID)
	if err != nil {
		return fmt.Errorf("failed to start keep-alive: %w", err)
	}

	go func() {
		for {
			// This loop consumes the keep-alive responses. If the channel is closed,
			// it means the lease has been revoked or has expired.
			ka, ok := <-keepAliveCh
			if !ok {
				r.logger.Warn("keep-alive channel closed, worker registration may have expired")
				return
			}
			r.logger.Debug("lease keep-alive refreshed", "lease_id", ka.ID, "ttl", ka.TTL)
		}
	}()

	r.logger.Info("worker registered successfully", "key", r.key, "value", r.value)
	return nil
}

// Deregister removes the worker's registration from etcd.
func (r *Registry) Deregister(ctx context.Context) error {
	r.logger.Info("deregistering worker", "key", r.key)
	
	// Revoke the lease, which will automatically delete the associated key.
	if _, err := r.client.Revoke(ctx, r.leaseID); err != nil {
		return fmt.Errorf("failed to revoke lease: %w", err)
	}
	return nil
}
