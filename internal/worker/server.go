// internal/worker/server.go
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"distributed-cron/internal/domain"
	"distributed-cron/internal/metrics"
	pb "distributed-cron/proto"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Server implements the proto.WorkerServer interface.
type Server struct {
	pb.UnimplementedWorkerServer
	executors map[domain.ExecutorType]domain.TaskExecutor
	locker    domain.Locker
	execRepo  domain.ExecutionRepository
	workerID  string // Add workerID to the server struct
	logger    *slog.Logger
	tracer    trace.Tracer
}

// NewServer creates a new gRPC server for the worker.
func NewServer(executors map[domain.ExecutorType]domain.TaskExecutor, locker domain.Locker, execRepo domain.ExecutionRepository, workerID string, logger *slog.Logger) *Server {
	return &Server{
		executors: executors,
		locker:    locker,
		execRepo:  execRepo,
		workerID:  workerID,
		logger:    logger.With("component", "grpc-server"),
		tracer:    otel.Tracer("distributed-cron-worker"),
	}
}

// ExecuteTask is the RPC method called by the master to run a task.
func (s *Server) ExecuteTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	ctx, span := s.tracer.Start(ctx, "worker.ExecuteTask.Accept")
	defer span.End()

	s.logger.Info("received task execution request", "job_name", req.Name)
	span.SetAttributes(attribute.String("job.name", req.Name))

	parentSpanContext := trace.SpanFromContext(ctx).SpanContext()

	job, err := s.protoToDomain(req)
	if err != nil {
		s.logger.Error("failed to convert proto request to domain job", "error", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid task request")
		return &pb.TaskResponse{ErrorMessage: err.Error()}, nil
	}

	executionID := uuid.NewString()

	go s.runJob(parentSpanContext, executionID, job)

	return &pb.TaskResponse{
		ExecutionId: executionID,
	}, nil
}

// runJob handles the actual execution logic in the background.
func (s *Server) runJob(parentSpanContext trace.SpanContext, executionID string, job *domain.Job) {
	ctx, span := s.tracer.Start(
		context.Background(),
		"worker.runJob",
		trace.WithLinks(trace.Link{SpanContext: parentSpanContext}),
		trace.WithAttributes(attribute.String("job.name", job.Name), attribute.String("execution.id", executionID)),
	)
	defer span.End()

	logger := s.logger.With("job_name", job.Name, "job_id", job.ID, "execution_id", executionID)

	// Create the initial execution record
	record := &domain.ExecutionRecord{
		ID:        executionID,
		JobName:   job.Name,
		StartTime: time.Now(),
		Status:    domain.ExecutionStatusRunning,
		WorkerID:  s.workerID,
	}

	// Save the initial "running" record
	if err := s.execRepo.Save(ctx, record); err != nil {
		logger.Error("failed to save initial execution record", "error", err)
		span.RecordError(err)
		// We still proceed with execution even if saving the record fails initially.
	}

	// Defer the final update of the execution record.
	defer func() {
		record.EndTime = time.Now()
		if r := recover(); r != nil {
			record.Status = domain.ExecutionStatusFailed
			record.Error = fmt.Sprintf("panic: %v", r)
			span.RecordError(fmt.Errorf(record.Error))
			span.SetStatus(codes.Error, "job execution panicked")
			logger.Error("job execution panicked", "panic", r)
		}

		// Save the final record state
		if err := s.execRepo.Save(context.Background(), record); err != nil {
			logger.Error("failed to save final execution record", "error", err)
			span.RecordError(err)
		}
	}()

	// The rest of the execution logic
	var execErr error
	defer func() {
		if execErr != nil {
			record.Status = domain.ExecutionStatusFailed
			record.Error = execErr.Error()
			metrics.JobExecutionTotal.WithLabelValues(job.Name, "failed").Inc()
			span.SetStatus(codes.Error, "job execution failed")
			span.RecordError(execErr)
		} else {
			record.Status = domain.ExecutionStatusSuccess
			metrics.JobExecutionTotal.WithLabelValues(job.Name, "success").Inc()
			span.SetStatus(codes.Ok, "job execution successful")
		}
	}()

	if job.ConcurrencyPolicy == domain.ConcurrencyPolicyForbid {
		lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		lock, err := s.locker.Lock(lockCtx, job.Name)
		if err != nil {
			execErr = fmt.Errorf("skipped execution: %w", err)
			logger.Warn(execErr.Error())
			span.AddEvent("skipped_execution", trace.WithAttributes(attribute.String("reason", "lock_not_acquired")))
			return // Exit before execution
		}
		logger.Info("acquired lock for job execution")
		span.AddEvent("lock_acquired")
		defer func() {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := lock.Unlock(unlockCtx); err != nil {
				logger.Error("failed to unlock job", "error", err)
			} else {
				logger.Info("released job lock")
			}
		}()
	}

	executor, ok := s.executors[job.ExecutorType]
	if !ok {
		execErr = fmt.Errorf("no executor found for type: %s", job.ExecutorType)
		logger.Error(execErr.Error())
		return
	}

	// 3. Execute the task.
	logger.Info("executing job")
	output, execErr := executor.Execute(ctx, job)
	record.Output = output
}

// protoToDomain converts a protobuf TaskRequest to a domain.Job object.
func (s *Server) protoToDomain(req *pb.TaskRequest) (*domain.Job, error) {
	// ... (This function remains the same as before)
	job := &domain.Job{
		ID:                req.Id,
		Name:              req.Name,
		CronExpr:          req.CronExpr,
		ExecutorType:      domain.ExecutorType(req.ExecutorType),
		ConcurrencyPolicy: domain.ConcurrencyPolicy(req.ConcurrencyPolicy),
		CreatedAt:         req.CreatedAt.AsTime(),
	}

	switch job.ExecutorType {
	case domain.ExecutorTypeHTTP:
		if req.HttpExecutor == nil {
			return nil, fmt.Errorf("http_executor is nil for http job type")
		}
		job.Executor = domain.JobExecutor{
			URL:    req.HttpExecutor.Url,
			Method: req.HttpExecutor.Method,
		}
	case domain.ExecutorTypeShell:
		if req.ShellExecutor == nil {
			return nil, fmt.Errorf("shell_executor is nil for shell job type")
		}
		job.Executor = domain.JobExecutor{
			Command: req.ShellExecutor.Command,
		}
	default:
		return nil, fmt.Errorf("unknown executor type: %s", req.ExecutorType)
	}

	if req.RetryPolicy != nil {
		backoff, err := time.ParseDuration(req.RetryPolicy.Backoff)
		if err != nil {
			return nil, fmt.Errorf("invalid backoff duration: %w", err)
		}
		job.RetryPolicy = &domain.RetryPolicy{
			MaxRetries: int(req.RetryPolicy.MaxRetries),
			Backoff:    backoff,
		}
	}
	return job, nil
}
