// internal/infra/etcd/etcd_job_repository.go
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"

	"distributed-cron/internal/domain"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	JobSaveDir = "/cron/jobs/"
)

type etcdJobRepository struct {
	client *clientv3.Client
	logger *slog.Logger
	tracer trace.Tracer
}

// NewEtcdJobRepository creates a new repository for jobs backed by etcd.
func NewEtcdJobRepository(client *clientv3.Client, logger *slog.Logger) domain.JobRepository {
	return &etcdJobRepository{
		client: client,
		logger: logger,
		tracer: otel.Tracer("distributed-cron-etcd-repo"),
	}
}

// Save persists the Job struct to etcd.
func (r *etcdJobRepository) Save(ctx context.Context, job *domain.Job) error {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.Save")
	defer span.End()

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job to JSON: %w", err)
	}

	key := path.Join(JobSaveDir, job.Name)
	span.SetAttributes(
		attribute.String("job.name", job.Name),
		attribute.String("etcd.key", key),
	)

	_, err = r.client.Put(ctx, key, string(jobJSON))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to put job to etcd")
		return fmt.Errorf("failed to save job %s to etcd: %w", job.Name, err)
	}
	return nil
}

// Delete removes a job from etcd.
func (r *etcdJobRepository) Delete(ctx context.Context, name string) error {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	key := path.Join(JobSaveDir, name)
	_, err := r.client.Delete(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete job from etcd")
		return fmt.Errorf("failed to delete job %s from etcd: %w", name, err)
	}
	return nil
}

// Get retrieves a job from etcd.
func (r *etcdJobRepository) Get(ctx context.Context, name string) (*domain.Job, error) {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.Get")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	key := path.Join(JobSaveDir, name)
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get job from etcd")
		return nil, fmt.Errorf("failed to get job %s from etcd: %w", name, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, domain.ErrJobNotFound
	}

	var job domain.Job
	if err := json.Unmarshal(resp.Kvs[0].Value, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job %s from JSON: %w", name, err)
	}
	return &job, nil
}

// List retrieves all jobs from etcd.
func (r *etcdJobRepository) List(ctx context.Context) ([]*domain.Job, error) {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.List")
	defer span.End()

	resp, err := r.client.Get(ctx, JobSaveDir, clientv3.WithPrefix())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list jobs from etcd")
		return nil, fmt.Errorf("failed to list jobs from etcd: %w", err)
	}
	span.SetAttributes(attribute.Int("etcd.kv_count", len(resp.Kvs)))


	jobs := make([]*domain.Job, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var job domain.Job
		if err := json.Unmarshal(kv.Value, &job); err != nil {
			r.logger.Warn("failed to unmarshal job from etcd", "key", string(kv.Key), "error", err)
			continue
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}