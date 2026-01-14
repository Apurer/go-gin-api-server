package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	workerlog "go.temporal.io/sdk/log"

	petstoreserver "github.com/GIT_USER_ID/GIT_REPO_ID/go"

	petsmemory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/memory"
	petsobs "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/observability"
	petspostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/persistence/postgres"
	petsworkflows "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/workflows"
	petsapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application"
	petsports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
	platformobservability "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/observability"
	platformpostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/postgres"

	storememory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/adapters/memory"
	storeapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/application"

	usermemory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/adapters/memory"
	userapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/application"
)

// Run boots the Petstore HTTP API with observability, repositories, and workflows wired.
func Run(ctx context.Context) error {
	const serviceName = "petstore-api"
	instruments, shutdown, err := platformobservability.Init(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to initialize observability: %w", err)
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
	corePetService := petsapp.NewService(petRepo)
	petService := petsobs.New(
		corePetService,
		petsobs.WithLogger(logger),
		petsobs.WithTracer(instruments.Tracer("internal.pets.application")),
		petsobs.WithMeter(instruments.Meter("internal.pets.application")),
	)
	var petWorkflows petsports.WorkflowOrchestrator = petsworkflows.NewInlinePetWorkflows(petService)
	if temporalClient, err := connectTemporalClient(instruments); err != nil {
		logger.Warn("Temporal workflows unavailable, running inline AddPet", slog.String("error", err.Error()))
	} else {
		defer temporalClient.Close()
		petWorkflows = petsworkflows.NewTemporalPetWorkflows(temporalClient)
		logger.Info("Temporal workflows enabled", slog.String("namespace", envOrDefault("TEMPORAL_NAMESPACE", client.DefaultNamespace)))
	}
	storeService := storeapp.NewService(storememory.NewRepository())
	userService := userapp.NewService(usermemory.NewRepository(), usermemory.NewSessionStore())

	handlers := petstoreserver.ApiHandleFunctions{
		PetAPI:   petstoreserver.NewPetAPI(petService, petWorkflows),
		StoreAPI: petstoreserver.NewStoreAPI(storeService),
		UserAPI:  petstoreserver.NewUserAPI(userService),
	}

	router := petstoreserver.NewRouter(handlers)
	router.Use(otelgin.Middleware(serviceName))
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	logger.Info("Petstore API listening", slog.String("addr", addr))
	if err := router.Run(addr); err != nil {
		logger.Error("Petstore API server exited", slog.String("addr", addr), slog.String("error", err.Error()))
		return err
	}
	return nil
}

func buildPetRepository(ctx context.Context, logger *slog.Logger) (petsports.Repository, func()) {
	dsn := os.Getenv("POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		logger.Warn("POSTGRES_DSN not set, falling back to in-memory pet repository")
		return petsmemory.NewRepository(), func() {}
	}
	db, err := platformpostgres.Connect(ctx, dsn)
	if err != nil {
		logger.Warn("failed to connect to postgres, falling back to memory", slog.String("error", err.Error()))
		return petsmemory.NewRepository(), func() {}
	}
	sqlDB, err := db.DB()
	if err != nil {
		logger.Warn("failed to unwrap postgres connection, falling back to memory", slog.String("error", err.Error()))
		return petsmemory.NewRepository(), func() {}
	}
	logger.Info("pet repository configured with postgres")
	return petspostgres.NewRepository(db), func() { _ = sqlDB.Close() }
}

func connectTemporalClient(instruments *platformobservability.Instruments) (client.Client, error) {
	if os.Getenv("TEMPORAL_DISABLED") == "1" {
		return nil, errors.New("temporal disabled via TEMPORAL_DISABLED env")
	}
	address := os.Getenv("TEMPORAL_ADDRESS")
	if address == "" {
		address = client.DefaultHostPort
	}
	tracerOptions := temporalotel.TracerOptions{}
	if instruments != nil {
		tracerOptions.Tracer = instruments.Tracer("temporal-client")
	}
	tracingInterceptor, err := temporalotel.NewTracingInterceptor(tracerOptions)
	if err != nil {
		return nil, err
	}
	logger := workerlog.NewStructuredLogger(effectiveLogger(instruments))
	options := client.Options{
		HostPort:  address,
		Namespace: envOrDefault("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		Logger:    logger,
	}
	options.Interceptors = append(options.Interceptors, tracingInterceptor)
	return client.Dial(options)
}

func effectiveLogger(instruments *platformobservability.Instruments) *slog.Logger {
	if instruments != nil && instruments.Logger != nil {
		return instruments.Logger
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
