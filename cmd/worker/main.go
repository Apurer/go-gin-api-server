package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	workerlog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	petsmemory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/memory"
	petsobs "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/observability"
	petspostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/persistence/postgres"
	petsapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application"
	petsports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
	platformobservability "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/observability"
	platformpostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/postgres"
	petactivities "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/temporal/activities/pets"
	petworkflows "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/temporal/workflows/pets"
	"gorm.io/gorm"
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

	db, cleanupRepo := platformpostgres.ConnectFromEnv(ctx, logger)
	defer cleanupRepo()
	petRepo := buildPetRepository(db, logger)
	corePetService := petsapp.NewService(petRepo)
	petService := petsobs.New(
		corePetService,
		petsobs.WithLogger(logger),
		petsobs.WithTracer(instruments.Tracer("internal.pets.application")),
		petsobs.WithMeter(instruments.Meter("internal.pets.application")),
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

func buildPetRepository(db *gorm.DB, logger *slog.Logger) petsports.Repository {
	if db == nil {
		logger.Warn("POSTGRES_DSN not set or unavailable, falling back to in-memory pet repository")
		return petsmemory.NewRepository()
	}
	logger.Info("worker pet repository configured with postgres")
	return petspostgres.NewRepository(db)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
