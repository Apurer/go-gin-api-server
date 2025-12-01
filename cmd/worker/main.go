package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	workerlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/ports"
	petsmemory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/repository/memory"
	petspostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/repository/postgres"
	petsservice "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/service"
	platformdb "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/db"
	platformobservability "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/observability"
	petactivities "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/temporal/activities/pets"
	petworkflows "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/temporal/workflows/pets"
)

func main() {
	ctx := context.Background()
	const serviceName = "petstore-worker"
	instruments, shutdown, err := platformobservability.Init(ctx, serviceName)
	if err != nil {
		log.Fatalf("failed to initialize observability: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			instruments.Logger.Error("failed to shutdown observability", slog.String("error", err.Error()))
		}
	}()
	logger := instruments.Logger

	petRepo, cleanupRepo := buildPetRepository(ctx, logger)
	defer cleanupRepo()
	petService := petsservice.NewService(
		petRepo,
		petsservice.WithLogger(logger),
		petsservice.WithTracer(instruments.Tracer("internal.pets.service")),
		petsservice.WithMeter(instruments.Meter("internal.pets.service")),
	)
	petActivities := petactivities.NewActivities(petService)

	tracerOptions := temporalotel.TracerOptions{Tracer: instruments.Tracer("temporal-worker")}
	tracingInterceptor, err := temporalotel.NewTracingInterceptor(tracerOptions)
	if err != nil {
		logger.Error("failed to configure Temporal tracing interceptor", slog.String("error", err.Error()))
		os.Exit(1)
	}
	clientOptions := client.Options{
		HostPort:  envOrDefault("TEMPORAL_ADDRESS", client.DefaultHostPort),
		Namespace: envOrDefault("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		Logger:    workerlog.NewStructuredLogger(logger),
	}
	clientOptions.Interceptors = append(clientOptions.Interceptors, tracingInterceptor)
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		logger.Error("failed to create Temporal client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer temporalClient.Close()

	w := worker.New(temporalClient, petworkflows.PetCreationTaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(petworkflows.PetCreationWorkflow, workflow.RegisterOptions{Name: petworkflows.PetCreationWorkflowName})
	w.RegisterActivityWithOptions(petActivities.CreatePet, activity.RegisterOptions{Name: petactivities.CreatePetActivityName})

	logger.Info("worker listening", slog.String("taskQueue", petworkflows.PetCreationTaskQueue), slog.String("namespace", clientOptions.Namespace))
	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Error("Temporal worker exited with error", slog.String("error", err.Error()))
		return
	}
	logger.Info("Temporal worker stopped")
}

func buildPetRepository(ctx context.Context, logger *slog.Logger) (ports.Repository, func()) {
	dsn := os.Getenv("POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		logger.Warn("POSTGRES_DSN not set, falling back to in-memory pet repository")
		return petsmemory.NewRepository(), func() {}
	}
	db, err := platformdb.Connect(ctx, dsn)
	if err != nil {
		logger.Warn("worker failed to connect to postgres (falling back to memory)", slog.String("error", err.Error()))
		return petsmemory.NewRepository(), func() {}
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Warn("worker failed to unwrap postgres connection (falling back to memory)", slog.String("error", err.Error()))
		return petsmemory.NewRepository(), func() {}
	}
	logger.Info("worker pet repository configured with postgres")
	return petspostgres.NewRepository(db), func() { _ = sqlDB.Close() }
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
