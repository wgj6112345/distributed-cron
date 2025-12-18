// internal/scheduler/cron_scheduler.go
package scheduler

import (
	"context"
	"log/slog"

	"distributed-cron/internal/domain"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// cronScheduler's responsibility is now purely to trigger tasks at the right time.
type cronScheduler struct {
	cron       *cron.Cron
	dispatcher domain.Dispatcher // Depends on the dispatcher interface
	jobs       map[string]cron.EntryID
	logger     *slog.Logger
	tracer     trace.Tracer
}

// NewCronScheduler now takes a dispatcher instead of an executor and locker.
func NewCronScheduler(dispatcher domain.Dispatcher, logger *slog.Logger) domain.Schedular {
	c := cron.New(cron.WithSeconds())
	return &cronScheduler{
		cron:       c,
		dispatcher: dispatcher,
		jobs:       make(map[string]cron.EntryID),
		logger:     logger.With("component", "cron-scheduler"),
		tracer:     otel.Tracer("distributed-cron-scheduler"),
	}
}

func (s *cronScheduler) Start(ctx context.Context) error {
	s.logger.Info("cron scheduler started")
	s.cron.Start()
	<-ctx.Done()
	s.logger.Info("cron scheduler stopping...")
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	s.logger.Info("cron scheduler stopped")
	return ctx.Err()
}

func (s *cronScheduler) Stop() {
	// Stop logic is handled by context cancellation in Start()
}

// AddJob adds a job to the scheduler.
func (s *cronScheduler) AddJob(job *domain.Job) error {
	if entryID, ok := s.jobs[job.Name]; ok {
		s.cron.Remove(entryID)
	}

	jobWrapper := &cronJobWrapper{
		job:        job,
		dispatcher: s.dispatcher,
		logger:     s.logger.With("job_name", job.Name),
		tracer:     s.tracer,
	}

	entryID, err := s.cron.AddJob(job.CronExpr, jobWrapper)
	if err != nil {
		s.logger.Error("failed to add job to cron", "job_name", job.Name, "error", err)
		return err
	}

	s.jobs[job.Name] = entryID
	s.logger.Info("added job to scheduler", "job_name", job.Name, "schedule", job.CronExpr)
	return nil
}

// RemoveJob removes a job from the scheduler.
func (s *cronScheduler) RemoveJob(name string) error {
	if entryID, ok := s.jobs[name]; ok {
		s.cron.Remove(entryID)
		delete(s.jobs, name)
		s.logger.Info("removed job from scheduler", "job_name", name)
	}
	return nil
}

// cronJobWrapper now only calls the dispatcher.
type cronJobWrapper struct {
	job        *domain.Job
	dispatcher domain.Dispatcher
	logger     *slog.Logger
	tracer     trace.Tracer
}

// Run is called by the cron library. Its only job is to dispatch the task.
func (w *cronJobWrapper) Run() {
	// Start a new trace for this background job execution.
	ctx, span := w.tracer.Start(context.Background(), "scheduler.Dispatch",
		trace.WithAttributes(
			attribute.String("job.name", w.job.Name),
			attribute.String("job.id", w.job.ID),
		))
	defer span.End()

	w.logger.Info("dispatching job")
	if err := w.dispatcher.DispatchTask(ctx, w.job); err != nil {
		w.logger.Error("failed to dispatch job", "error", err)
		span.RecordError(err)
	}
}
