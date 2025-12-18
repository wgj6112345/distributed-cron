// internal/infra/shell/shell_task_executor.go
package shell

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"distributed-cron/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// shellTaskExecutor implements domain.TaskExecutor for shell commands.
type shellTaskExecutor struct {
	logger *slog.Logger
	tracer trace.Tracer
}

// NewShellTaskExecutor creates a new shellTaskExecutor instance.
func NewShellTaskExecutor(logger *slog.Logger) domain.TaskExecutor {
	return &shellTaskExecutor{
		logger: logger.With("executor_type", "shell"),
		tracer: otel.Tracer("distributed-cron-shell-executor"),
	}
}

// Execute runs the shell command specified in the job and returns its output.
func (e *shellTaskExecutor) Execute(ctx context.Context, job *domain.Job) (string, error) {
	ctx, span := e.tracer.Start(ctx, "executor.shell.Execute",
		trace.WithAttributes(
			attribute.String("job.name", job.Name),
			attribute.String("job.command", job.Executor.Command),
		))
	defer span.End()

	e.logger.Info("executing shell command", "command", job.Executor.Command, "job_name", job.Name)

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", job.Executor.Command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	errOutput := stderr.String()

	if output != "" {
		span.SetAttributes(attribute.String("shell.stdout", output))
	}
	if errOutput != "" {
		span.SetAttributes(attribute.String("shell.stderr", errOutput))
		// Prepend stderr to the main output for visibility
		if output != "" {
			output = fmt.Sprintf("[STDERR]:\n%s\n[STDOUT]:\n%s", errOutput, output)
		} else {
			output = fmt.Sprintf("[STDERR]:\n%s", errOutput)
		}
	}

	if err != nil {
		span.SetStatus(codes.Error, "shell command failed")
		span.RecordError(err)
		return output, fmt.Errorf("shell command failed: %w", err)
	}

	e.logger.Info("shell command executed successfully", "job_name", job.Name)
	return output, nil
}
