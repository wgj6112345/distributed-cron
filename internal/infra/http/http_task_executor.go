package http

import (
	"context"
	"distributed-cron/internal/domain"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type httpTaskExecutor struct {
	client *http.Client
}

func NewHttpTaskExecutor() domain.TaskExecutor {
	return &httpTaskExecutor{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Execute initiates an HTTP request and retries on failure.
func (e *httpTaskExecutor) Execute(ctx context.Context, job *domain.Job) (string, error) {
	if job.RetryPolicy == nil || job.RetryPolicy.MaxRetries == 0 {
		return e.doExecute(ctx, job)
	}

	var lastErr error
	var output string
	for i := 0; i <= job.RetryPolicy.MaxRetries; i++ {
		output, err := e.doExecute(ctx, job)
		if err == nil {
			return output, nil
		}

		lastErr = err

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// Retriable
		} else if strings.Contains(err.Error(), "5xx") {
			// Retriable
		} else {
			return output, fmt.Errorf("non-retriable error on attempt %d: %w", i+1, err)
		}

		if i == job.RetryPolicy.MaxRetries {
			break
		}

		time.Sleep(job.RetryPolicy.Backoff)
	}

	return output, fmt.Errorf("job failed after %d retries: %w", job.RetryPolicy.MaxRetries, lastErr)
}

// doExecute performs a single HTTP request execution.
func (e *httpTaskExecutor) doExecute(ctx context.Context, job *domain.Job) (string, error) {
	req, err := http.NewRequestWithContext(ctx, job.Executor.Method, job.Executor.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read a small portion of the body for output logging.
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // Read max 1KB

	if resp.StatusCode >= 500 {
		return string(bodyBytes), fmt.Errorf("http request returned 5xx server error: %s", resp.Status)
	}
	if resp.StatusCode >= 400 {
		return string(bodyBytes), fmt.Errorf("http request returned 4xx client error: %s", resp.Status)
	}

	return string(bodyBytes), nil
}
