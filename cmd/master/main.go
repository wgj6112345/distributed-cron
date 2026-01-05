// cmd/master/main.go
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	http_api "distributed-cron/internal/api/http"
	"distributed-cron/internal/config"
	"distributed-cron/internal/infra/etcd"
	"distributed-cron/internal/master"
	"distributed-cron/internal/scheduler"
	"distributed-cron/internal/tracing"
	"distributed-cron/internal/usecase"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// corsMiddleware wraps an http.Handler with CORS headers for local development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // For local dev, allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle pre-flight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

func main() {
	// 1. Initialize logger and tracer
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	tracerShutdown, err := tracing.InitTracer("distributed-cron-master")
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tracerShutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown tracer: %v", err)
		}
	}()

	log.Println("Starting distributed cron master node...")

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	nodeID := uuid.New().String()
	log.Printf("Node ID: %s", nodeID)

	// 3. Create root context for lifecycle management
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Setup graceful shutdown
	setupGracefulShutdown(cancel)

	// 5. Init etcd client
	etcdClient, err := etcd.NewClient(cfg.EtcdEndpoints, cfg.EtcdTimeout)
	if err != nil {
		log.Fatalf("Failed to create etcd client: %v", err)
	}
	defer etcdClient.Close()
	log.Println("Connected to etcd.")

	// 6. Instantiate components
	discovery := master.NewWorkerDiscovery(etcdClient, logger)
	dispatcher := master.NewDispatcher(discovery, logger)
	jobRepo := etcd.NewEtcdJobRepository(etcdClient, logger)
	execRepo := etcd.NewEtcdExecutionRepository(etcdClient, logger)

	go discovery.WatchWorkers(rootCtx)

	cronScheduler := scheduler.NewCronScheduler(dispatcher, logger)
	jobService := usecase.NewJobService(jobRepo, execRepo, cronScheduler, logger)
	leaderManager := etcd.NewEtcdLeaderElectionManager(etcdClient, nodeID, cfg.EtcdTimeout, logger)
	schedulerService := usecase.NewSchedularService(leaderManager, cronScheduler, jobRepo, nodeID) // leaderManager is not used here directly

	jobHandler := http_api.NewJobHandler(jobService, logger)

	// 10. Register routes and metrics endpoint
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	jobHandler.RegisterRoutes(mux)

	// 11. Start SchedulerService
	go func() {
		if err := schedulerService.Start(rootCtx); err != nil {
			log.Fatalf("SchedulerService stopped with error: %v", err)
		}
	}()

	// 12. Start HTTP API server with CORS middleware
	log.Printf("Starting HTTP API server on %s", cfg.HttpListenAddr)
	server := &http.Server{
		Addr:    cfg.HttpListenAddr,
		Handler: corsMiddleware(mux), // Apply CORS middleware
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// 13. Block until shutdown
	<-rootCtx.Done()
	log.Println("Shutting down application gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP server shutdown failed: %v", err)
	}

	log.Println("Application shut down.")
}

func setupGracefulShutdown(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v. Initiating graceful shutdown...", sig)
		cancel()
	}()
}
