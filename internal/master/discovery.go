// internal/master/discovery.go
package master

import (
	"context"
	"log/slog"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// WorkerRegistryPrefix is the etcd prefix where workers register themselves.
	WorkerRegistryPrefix = "/cron/workers/"
)

// WorkerDiscovery handles discovering and tracking available worker nodes.
type WorkerDiscovery struct {
	client  *clientv3.Client
	logger  *slog.Logger
	workers map[string]string // map of workerID -> workerAddr
	mu      sync.RWMutex
}

// NewWorkerDiscovery creates a new discovery service.
func NewWorkerDiscovery(client *clientv3.Client, logger *slog.Logger) *WorkerDiscovery {
	return &WorkerDiscovery{
		client:  client,
		logger:  logger.With("component", "worker-discovery"),
		workers: make(map[string]string),
	}
}

// WatchWorkers starts watching etcd for worker registrations and deregistrations.
// This is a blocking call and should be run in a goroutine.
func (d *WorkerDiscovery) WatchWorkers(ctx context.Context) {
	d.logger.Info("starting to watch for workers")

	// 1. Initial load of all existing workers
	if err := d.loadInitialWorkers(ctx); err != nil {
		d.logger.Error("failed to perform initial worker load", "error", err)
	}

	// 2. Set up a watch for future changes
	watchChan := d.client.Watch(ctx, WorkerRegistryPrefix, clientv3.WithPrefix())

	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			workerID := string(event.Kv.Key)
			workerAddr := string(event.Kv.Value)

			d.mu.Lock()
			switch event.Type {
			case clientv3.EventTypePut:
				// A new worker registered or an existing one's lease was updated
				if _, ok := d.workers[workerID]; !ok {
					d.logger.Info("new worker discovered", "id", workerID, "addr", workerAddr)
				}
				d.workers[workerID] = workerAddr
			case clientv3.EventTypeDelete:
				// A worker deregistered (lease expired or graceful shutdown)
				d.logger.Info("worker deregistered", "id", workerID, "addr", d.workers[workerID])
				delete(d.workers, workerID)
			}
			d.mu.Unlock()
		}
	}
	d.logger.Info("stopped watching for workers")
}

func (d *WorkerDiscovery) loadInitialWorkers(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := d.client.Get(ctx, WorkerRegistryPrefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for _, kv := range resp.Kvs {
		workerID := string(kv.Key)
		workerAddr := string(kv.Value)
		d.logger.Info("found existing worker", "id", workerID, "addr", workerAddr)
		d.workers[workerID] = workerAddr
	}
	return nil
}


// GetWorkers returns a snapshot of the current available worker addresses.
func (d *WorkerDiscovery) GetWorkers() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	addrs := make([]string, 0, len(d.workers))
	for _, addr := range d.workers {
		addrs = append(addrs, addr)
	}
	return addrs
}
