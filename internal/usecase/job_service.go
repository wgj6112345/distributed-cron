package usecase

import (
	"context"
	"log/slog"
	"time"

	"distributed-cron/internal/domain"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// jobService 实现了对 Job 的核心业务逻辑操作。
type JobService struct {
	repo      domain.JobRepository
	execRepo  domain.ExecutionRepository // Add dependency for execution records
	scheduler domain.Schedular
	logger    *slog.Logger
	tracer    trace.Tracer
}

// NewJobService creates a new JobService instance.
func NewJobService(repo domain.JobRepository, execRepo domain.ExecutionRepository, scheduler domain.Schedular, logger *slog.Logger) *JobService {
	return &JobService{
		repo:      repo,
		execRepo:  execRepo,
		scheduler: scheduler,
		logger:    logger,
		tracer:    otel.Tracer("distributed-cron-usecase"),
	}
}

// ... (Save, Delete, Get, List methods remain the same) ...

// ListHistory lists the execution history for a specific job.
func (s *JobService) ListHistory(ctx context.Context, jobName string, page, pageSize int) ([]*domain.ExecutionRecord, error) {
	ctx, span := s.tracer.Start(ctx, "service.ListHistory")
	defer span.End()
	span.SetAttributes(
		attribute.String("job.name", jobName),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	)

	records, err := s.execRepo.ListByJobName(ctx, jobName, page, pageSize)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list job history from repository")
	}
	return records, err
}

// Save 处理保存一个任务的业务逻辑。
func (s *JobService) Save(ctx context.Context, job *domain.Job) error {
	ctx, span := s.tracer.Start(ctx, "service.Save")
	defer span.End()

	if err := job.Validate(); err != nil {
		return err
	}

	now := time.Now()
	if job.ID == "" {
		job.ID = uuid.New().String()
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	span.SetAttributes(attribute.String("job.id", job.ID), attribute.String("job.name", job.Name))

	if err := s.repo.Save(ctx, job); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to save job to repository")
		return err
	}

	if err := s.scheduler.AddJob(job); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to add job to scheduler")
		return err
	}

	return nil
}

// Delete 处理删除一个任务的业务逻辑。
func (s *JobService) Delete(ctx context.Context, name string) error {
	ctx, span := s.tracer.Start(ctx, "service.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	if err := s.scheduler.RemoveJob(name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to remove job from scheduler")
		return err
	}

	if err := s.repo.Delete(ctx, name); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete job from repository")
		return err
	}
	return nil
}

// Get 获取一个任务。
func (s *JobService) Get(ctx context.Context, name string) (*domain.Job, error) {
	ctx, span := s.tracer.Start(ctx, "service.Get")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	job, err := s.repo.Get(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get job from repository")
	}
	return job, err
}

// List 列出所有任务。
func (s *JobService) List(ctx context.Context) ([]*domain.Job, error) {
	ctx, span := s.tracer.Start(ctx, "service.List")
	defer span.End()

	jobs, err := s.repo.List(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list jobs from repository")
	}
	return jobs, err
}
