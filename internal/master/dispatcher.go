// internal/master/dispatcher.go
package master

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"

	"distributed-cron/internal/domain"
	pb "distributed-cron/proto"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Dispatcher handles dispatching tasks to available workers.
type Dispatcher struct {
	discovery *WorkerDiscovery
	clients   map[string]pb.WorkerClient // A cache for gRPC clients
	mu        sync.Mutex
	logger    *slog.Logger
}

// NewDispatcher creates a new task dispatcher.
func NewDispatcher(discovery *WorkerDiscovery, logger *slog.Logger) domain.Dispatcher {
	return &Dispatcher{
		discovery: discovery,
		clients:   make(map[string]pb.WorkerClient),
		logger:    logger.With("component", "dispatcher"),
	}
}

// DispatchTask selects a worker and sends the task via gRPC.
func (d *Dispatcher) DispatchTask(ctx context.Context, job *domain.Job) error {
	// 1. Get available workers from the discovery service.
	workers := d.discovery.GetWorkers()
	if len(workers) == 0 {
		return fmt.Errorf("no available workers to dispatch job %s", job.Name)
	}

	// 2. Select a worker (simple random selection for MVP).
	workerAddr := workers[rand.Intn(len(workers))]

	d.logger.Info("dispatching task to worker", "job_name", job.Name, "worker_addr", workerAddr)

	// 3. Get or create a gRPC client for the selected worker.
	client, err := d.getOrCreateClient(workerAddr)
	if err != nil {
		return err
	}

	// 4. Convert domain.Job to a protobuf TaskRequest.
	taskReq, err := d.domainToProto(job)
	if err != nil {
		return err
	}

	// 5. Call the worker's ExecuteTask RPC.
	// The context passed here will propagate trace information.
	_, err = client.ExecuteTask(ctx, taskReq)
	if err != nil {
		d.logger.Error("failed to execute task via gRPC", "job_name", job.Name, "worker_addr", workerAddr, "error", err)
		return err
	}

	return nil
}

func (d *Dispatcher) getOrCreateClient(addr string) (pb.WorkerClient, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If client already exists in cache, return it.
	if client, ok := d.clients[addr]; ok {
		return client, nil
	}

	// Otherwise, create a new gRPC connection.
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Add OpenTelemetry Stats Handler for automatic trace propagation.
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to worker at %s: %w", addr, err)
	}

	client := pb.NewWorkerClient(conn)
	d.clients[addr] = client
	d.logger.Info("created new gRPC client for worker", "addr", addr)

	return client, nil
}

// domainToProto converts a domain.Job object to a proto.TaskRequest object.
func (d *Dispatcher) domainToProto(job *domain.Job) (*pb.TaskRequest, error) {
	req := &pb.TaskRequest{
		Id:                job.ID,
		Name:              job.Name,
		CronExpr:          job.CronExpr,
		ExecutorType:      string(job.ExecutorType),
		ConcurrencyPolicy: string(job.ConcurrencyPolicy),
		CreatedAt:         timestamppb.New(job.CreatedAt),
	}

	switch job.ExecutorType {
	case domain.ExecutorTypeHTTP:
		req.HttpExecutor = &pb.ExecutorHttp{
			Url:    job.Executor.URL,
			Method: job.Executor.Method,
		}
	case domain.ExecutorTypeShell:
		req.ShellExecutor = &pb.ExecutorShell{
			Command: job.Executor.Command,
		}
	default:
		return nil, fmt.Errorf("unknown executor type: %s", job.ExecutorType)
	}

	if job.RetryPolicy != nil {
		req.RetryPolicy = &pb.RetryPolicy{
			MaxRetries: int32(job.RetryPolicy.MaxRetries),
			Backoff:    job.RetryPolicy.Backoff.String(),
		}
	}

	return req, nil
}
