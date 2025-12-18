// internal/infra/etcd/etcd_execution_repository.go
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
	ExecutionHistoryDir = "/cron/history/"
)

type etcdExecutionRepository struct {
	client *clientv3.Client
	logger *slog.Logger
	tracer trace.Tracer
}

// NewEtcdExecutionRepository creates a new repository for execution records backed by etcd.
func NewEtcdExecutionRepository(client *clientv3.Client, logger *slog.Logger) domain.ExecutionRepository {
	return &etcdExecutionRepository{
		client: client,
		logger: logger,
		tracer: otel.Tracer("distributed-cron-etcd-execution-repo"),
	}
}

// Save persists a single execution record to etcd.
// The key is structured as /cron/history/{jobName}/{executionID}.
func (r *etcdExecutionRepository) Save(ctx context.Context, record *domain.ExecutionRecord) error {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.SaveExecution")
	defer span.End()

	recordJSON, err := json.Marshal(record)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal execution record")
		return fmt.Errorf("failed to marshal execution record %s to JSON: %w", record.ID, err)
	}

	key := path.Join(ExecutionHistoryDir, record.JobName, record.ID)
	span.SetAttributes(
		attribute.String("execution.id", record.ID),
		attribute.String("job.name", record.JobName),
		attribute.String("etcd.key", key),
	)

	_, err = r.client.Put(ctx, key, string(recordJSON))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to put execution record to etcd")
		return fmt.Errorf("failed to save execution record %s to etcd: %w", record.ID, err)
	}
	return nil
}

// Get retrieves a single execution record by its JobName and ExecutionID.
func (r *etcdExecutionRepository) Get(ctx context.Context, jobName, executionID string) (*domain.ExecutionRecord, error) {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.GetExecution")
	defer span.End()
	span.SetAttributes(
		attribute.String("job.name", jobName),
		attribute.String("execution.id", executionID),
	)

	key := path.Join(ExecutionHistoryDir, jobName, executionID)
	resp, err := r.client.Get(ctx, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get execution record from etcd")
		return nil, fmt.Errorf("failed to get execution record %s/%s from etcd: %w", jobName, executionID, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("execution record %s/%s not found", jobName, executionID)
	}

	var record domain.ExecutionRecord
	if err := json.Unmarshal(resp.Kvs[0].Value, &record); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal execution record")
		return nil, fmt.Errorf("failed to unmarshal execution record %s/%s from JSON: %w", jobName, executionID, err)
	}
	return &record, nil
}

// ListByJobName retrieves historical execution records for a specific job, with pagination.
// Records are returned in reverse chronological order (newest first).
func (r *etcdExecutionRepository) ListByJobName(ctx context.Context, jobName string, page, pageSize int) ([]*domain.ExecutionRecord, error) {
	ctx, span := r.tracer.Start(ctx, "repo.etcd.ListExecutions")
	defer span.End()
	span.SetAttributes(
		attribute.String("job.name", jobName),
		attribute.Int("page", page),
		attribute.Int("page_size", pageSize),
	)

	prefix := path.Join(ExecutionHistoryDir, jobName) + "/"
	resp, err := r.client.Get(ctx, prefix,
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByCreateRevision, clientv3.SortDescend), // Newest first
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list execution records from etcd")
		return nil, fmt.Errorf("failed to list execution records for job %s from etcd: %w", jobName, err)
	}

	records := make([]*domain.ExecutionRecord, 0, len(resp.Kvs))
	// Manual pagination for now. Etcd Get with Limit/Offset is for key-count, not index-based.
	// For large number of records, this needs more robust pagination (e.g., cursor-based).
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	for i, kv := range resp.Kvs {
		if i < startIdx {
			continue // Skip records before the start of the current page
		}
		if i >= endIdx {
			break // Stop once we have enough records for the current page
		}

		var record domain.ExecutionRecord
		if err := json.Unmarshal(kv.Value, &record); err != nil {
			r.logger.Warn("failed to unmarshal execution record from etcd", "key", string(kv.Key), "error", err)
			continue
		}
		records = append(records, &record)
	}
	span.SetAttributes(attribute.Int("records_returned", len(records)))
	return records, nil
}
