// internal/api/http/job_handler.go
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"distributed-cron/internal/domain"
	"distributed-cron/internal/metrics"
	"distributed-cron/internal/usecase"

	"github.com/go-playground/validator/v10"
	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes" // Correct import
	"go.opentelemetry.io/otel/trace"
)

// JobHandler 负责处理与 Job 相关的 HTTP 请求。
type JobHandler struct {
	service  *usecase.JobService
	logger   *slog.Logger
	validate *validator.Validate
	tracer   trace.Tracer
}

// NewJobHandler 创建一个新的 JobHandler，并初始化 validator。
func NewJobHandler(service *usecase.JobService, logger *slog.Logger) *JobHandler {
	validate := validator.New()

	_ = validate.RegisterValidation("cron", func(fl validator.FieldLevel) bool {
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		_, err := parser.Parse(fl.Field().String())
		return err == nil
	})

	_ = validate.RegisterValidation("duration", func(fl validator.FieldLevel) bool {
		_, err := time.ParseDuration(fl.Field().String())
		return err == nil
	})

	return &JobHandler{
		service:  service,
		logger:   logger.With("component", "job-handler"),
		validate: validate,
		tracer:   otel.Tracer("distributed-cron-api"),
	}
}

// A helper struct to capture the status code
type instrumentedResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *instrumentedResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RegisterRoutes registers job-related routes to the http.ServeMux.
func (h *JobHandler) RegisterRoutes(mux *http.ServeMux) {
	baseHandler := http.HandlerFunc(h.handleJobs)

	instrumentedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/jobs/"
		if jobName := strings.TrimPrefix(r.URL.Path, "/jobs/"); jobName != "" {
			path = "/jobs/{name}"
		}

		ctx, span := h.tracer.Start(r.Context(), "HTTP "+r.Method+" "+path, trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.target", r.URL.Path),
		))
		defer span.End()

		r = r.WithContext(ctx)

		iw := &instrumentedResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		baseHandler.ServeHTTP(iw, r)

		metrics.HttpRequestsTotal.WithLabelValues(path, r.Method, strconv.Itoa(iw.statusCode)).Inc()

		span.SetAttributes(attribute.Int("http.status_code", iw.statusCode))
		if iw.statusCode >= 500 {
			span.SetStatus(codes.Error, "Server Error")
		}
	})

	mux.Handle("/jobs/", instrumentedHandler)
}

// handleJobs is a general dispatcher for /jobs/ path
func (h *JobHandler) handleJobs(w http.ResponseWriter, r *http.Request) {
	// e.g. /jobs/my-job/history -> ["jobs", "my-job", "history"]
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(pathParts) < 1 || pathParts[0] != "jobs" {
		http.NotFound(w, r)
		return
	}

	var jobName, action string
	if len(pathParts) > 1 {
		jobName = pathParts[1]
	}
	if len(pathParts) > 2 {
		action = pathParts[2]
	}

	switch r.Method {
	case http.MethodGet:
		if jobName != "" && action == "history" {
			h.handleGetJobHistory(w, r, jobName)
		} else if jobName != "" && action == "" {
			h.handleGetJob(w, r, jobName)
		} else if jobName == "" && action == "" {
			h.handleListJobs(w, r)
		} else {
			http.NotFound(w, r)
		}
	case http.MethodPost, http.MethodPut:
		h.handleSaveJob(w, r)
	case http.MethodDelete:
		if jobName != "" && action == "" {
			h.handleDeleteJob(w, r, jobName)
		} else {
			http.Error(w, "Job name is required for deletion", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ... (handleSaveJob, handleDeleteJob, handleGetJob, handleListJobs remain the same) ...

// handleGetJobHistory handles listing execution history for a job (GET /jobs/{name}/history)
func (h *JobHandler) handleGetJobHistory(w http.ResponseWriter, r *http.Request, name string) {
	ctx, span := h.tracer.Start(r.Context(), "handler.GetJobHistory")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20 // default and max page size
	}
	span.SetAttributes(attribute.Int("page", page), attribute.Int("page_size", pageSize))


	history, err := h.service.ListHistory(ctx, name, page, pageSize)
	if err != nil {
		h.logger.Error("error listing job history", "job_name", name, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// handleSaveJob now uses DTO and validation
func (h *JobHandler) handleSaveJob(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.SaveJob")
	defer span.End()

	var req SaveJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetStatus(codes.Error, "Failed to decode request body")
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		span.SetStatus(codes.Error, "Validation failed")
		span.RecordError(err)
		var validationErrors []string
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors,
				"Field '"+err.Field()+"' failed on the '"+err.Tag()+"' tag.",
			)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Validation failed",
			"details": validationErrors,
		})
		return
	}

	job := req.ToDomainJob()
	span.SetAttributes(attribute.String("job.name", job.Name))

	if err := h.service.Save(ctx, job); err != nil {
		span.SetStatus(codes.Error, "Failed to save job in service")
		span.RecordError(err)
		h.logger.Error("error saving job", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

func (h *JobHandler) handleDeleteJob(w http.ResponseWriter, r *http.Request, name string) {
	ctx, span := h.tracer.Start(r.Context(), "handler.DeleteJob")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	if err := h.service.Delete(ctx, name); err != nil {
		span.SetStatus(codes.Error, "Failed to delete job in service")
		span.RecordError(err)
		h.logger.Error("error deleting job", "job_name", name, "error", err)
		http.Error(w, "Internal server error or job not found", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *JobHandler) handleGetJob(w http.ResponseWriter, r *http.Request, name string) {
	ctx, span := h.tracer.Start(r.Context(), "handler.GetJob")
	defer span.End()
	span.SetAttributes(attribute.String("job.name", name))

	job, err := h.service.Get(ctx, name)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to get job from service")
		span.RecordError(err)
		h.logger.Warn("error getting job", "job_name", name, "error", err)
		if errors.Is(err, domain.ErrJobNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (h *JobHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "handler.ListJobs")
	defer span.End()

	jobs, err := h.service.List(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to list jobs from service")
		span.RecordError(err)
		h.logger.Error("error listing jobs", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}
