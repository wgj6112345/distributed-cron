// cmd/worker/main.go
package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-cron/internal/config"
	"distributed-cron/internal/domain"
	"distributed-cron/internal/infra/etcd"
	http_infra "distributed-cron/internal/infra/http"
	shell_infra "distributed-cron/internal/infra/shell"
	"distributed-cron/internal/worker"
	pb "distributed-cron/proto"

	"github.com/google/uuid"
	otelgrpc "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	// 1. Init logger, config, etc.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// For worker, we might need its own address from config or env
	workerID := uuid.New().String()
	grpcListenAddr := ":50052" // Worker listens on this port
	log.Printf("Starting worker node %s, listening on %s", workerID, grpcListenAddr)

	// 2. Create root context for lifecycle management
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Setup graceful shutdown
	setupGracefulShutdown(cancel)

	// 4. Init etcd client
	etcdClient, err := etcd.NewClient(cfg.EtcdEndpoints, cfg.EtcdTimeout)
	if err != nil {
		log.Fatalf("Failed to create etcd client: %v", err)
	}
	defer etcdClient.Close()
	log.Println("Connected to etcd.")

	// 5. Register this worker in etcd
	registry := worker.NewRegistry(etcdClient, logger)
	regCtx, regCancel := context.WithTimeout(rootCtx, 5*time.Second)
	defer regCancel()
	err = registry.Register(regCtx, workerID, grpcListenAddr, int64(cfg.LeaderElectionTTL.Seconds()))
	if err != nil {
		log.Fatalf("Failed to register worker: %v", err)
	}

	defer func() {
		deregCtx, deregCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer deregCancel()
		if err := registry.Deregister(deregCtx); err != nil {
			logger.Error("failed to deregister worker", "error", err)
		}
	}()

	// 6. Instantiate executors, locker, and execution repository
	httpExecutor := http_infra.NewHttpTaskExecutor()
	shellExecutor := shell_infra.NewShellTaskExecutor(logger)
	locker := etcd.NewEtcdLocker(etcdClient)
	execRepo := etcd.NewEtcdExecutionRepository(etcdClient, logger) // Instantiate execution repository
	executors := map[domain.ExecutorType]domain.TaskExecutor{
		domain.ExecutorTypeHTTP:  httpExecutor,
		domain.ExecutorTypeShell: shellExecutor,
	}

	// 7. Instantiate and start the gRPC server
	lis, err := net.Listen("tcp", grpcListenAddr)
	if err != nil {
		log.Fatalf("Failed to listen for gRPC: %v", err)
	}

	workerServer := worker.NewServer(executors, locker, execRepo, workerID, logger) // Inject execRepo
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterWorkerServer(grpcServer, workerServer)

	log.Printf("gRPC server listening on %s", grpcListenAddr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// 8. Block until shutdown signal
	<-rootCtx.Done()
	log.Println("Shutting down worker node gracefully...")

	grpcServer.GracefulStop()

	log.Println("Worker node shut down.")
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
