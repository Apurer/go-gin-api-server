package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	workerlog "go.temporal.io/sdk/log"
	"gorm.io/gorm"

	petstoreserver "github.com/Apurer/go-gin-api-server/generated/go"

	partnerclient "github.com/Apurer/go-gin-api-server/internal/clients/http/partner"
	petspartner "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/external/partner"
	petsmemory "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/memory"
	petsobs "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/observability"
	petspostgres "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/persistence/postgres"
	petsworkflows "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/workflows"
	petsapp "github.com/Apurer/go-gin-api-server/internal/domains/pets/application"
	petsports "github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
	storeobs "github.com/Apurer/go-gin-api-server/internal/domains/store/adapters/observability"
	storepostgres "github.com/Apurer/go-gin-api-server/internal/domains/store/adapters/persistence/postgres"
	platformmigrations "github.com/Apurer/go-gin-api-server/internal/platform/migrations"
	platformobservability "github.com/Apurer/go-gin-api-server/internal/platform/observability"
	platformpostgres "github.com/Apurer/go-gin-api-server/internal/platform/postgres"

	storememory "github.com/Apurer/go-gin-api-server/internal/domains/store/adapters/memory"
	storeapp "github.com/Apurer/go-gin-api-server/internal/domains/store/application"
	storeports "github.com/Apurer/go-gin-api-server/internal/domains/store/ports"

	usermemory "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/memory"
	userobs "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/observability"
	userpostgres "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/persistence/postgres"
	userapp "github.com/Apurer/go-gin-api-server/internal/domains/users/application"
	userports "github.com/Apurer/go-gin-api-server/internal/domains/users/ports"
)

// Run boots the Petstore HTTP API with observability, repositories, and workflows wired.
func Run(ctx context.Context) error {
	const serviceName = "petstore-api"
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
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

	db, cleanupDB := connectPostgresWithConfig(ctx, logger, cfg.PostgresDSN)
	defer cleanupDB()
	if db != nil {
		if err := platformmigrations.Run(db); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
	}

	petRepo := buildPetRepository(db)
	partnerSync := buildPartnerSync(cfg.PartnerAPIBaseURL, logger)
	corePetService := petsapp.NewService(petRepo, petsapp.WithPartnerSync(partnerSync))
	petService := petsobs.New(
		corePetService,
		petsobs.WithLogger(logger),
		petsobs.WithTracer(instruments.Tracer("internal.pets.application")),
		petsobs.WithMeter(instruments.Meter("internal.pets.application")),
	)
	storeRepo := buildStoreRepository(db)
	storeService := storeobs.New(
		storeapp.NewService(storeRepo),
		storeobs.WithLogger(logger),
		storeobs.WithTracer(instruments.Tracer("internal.store.application")),
		storeobs.WithMeter(instruments.Meter("internal.store.application")),
	)

	userRepo := buildUserRepository(db)
	userSessionStore := buildUserSessionStore(db, cfg.SessionTTL)
	userService := userobs.New(
		userapp.NewService(userRepo, userSessionStore),
		userobs.WithLogger(logger),
		userobs.WithTracer(instruments.Tracer("internal.users.application")),
		userobs.WithMeter(instruments.Meter("internal.users.application")),
	)
	startSessionPurger(ctx, logger, userSessionStore, cfg.SessionPurgeIntervalMinute)

	var petWorkflows petsports.WorkflowOrchestrator
	var temporalClient client.Client
	if cfg.TemporalDisabled {
		logger.Warn("Temporal disabled via config, running inline AddPet")
		petWorkflows = petsworkflows.NewInlinePetWorkflows(petService)
	} else {
		c, err := connectTemporalClient(instruments, cfg)
		if err != nil {
			return fmt.Errorf("connect temporal client: %w", err)
		}
		temporalClient = c
		defer temporalClient.Close()
		petWorkflows = petsworkflows.NewTemporalPetWorkflows(temporalClient)
		logger.Info("Temporal workflows enabled", slog.String("namespace", cfg.TemporalNamespace))
	}

	handlers := petstoreserver.ApiHandleFunctions{
		PetAPI:   petstoreserver.NewPetAPI(petService, petWorkflows),
		StoreAPI: petstoreserver.NewStoreAPI(storeService),
		UserAPI:  petstoreserver.NewUserAPI(userService),
	}

	router := petstoreserver.NewRouter(handlers)
	router.Use(otelgin.Middleware(serviceName))
	registerHealthRoutes(router, cfg, db, temporalClient)
	addr := ":" + cfg.Port
	logger.Info("Petstore API listening", slog.String("addr", addr))
	if err := router.Run(addr); err != nil {
		logger.Error("Petstore API server exited", slog.String("addr", addr), slog.String("error", err.Error()))
		return err
	}
	return nil
}

func buildPetRepository(db *gorm.DB) petsports.Repository {
	if db == nil {
		return petsmemory.NewRepository()
	}
	return petspostgres.NewRepository(db)
}

func buildPartnerSync(baseURL string, logger *slog.Logger) petsports.PartnerSync {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}
	if logger != nil {
		logger.Info("partner sync enabled", slog.String("base_url", baseURL))
	}
	client := partnerclient.NewClient(baseURL, nil)
	return petspartner.NewSyncer(client)
}

func buildStoreRepository(db *gorm.DB) storeports.Repository {
	if db == nil {
		return storememory.NewRepository()
	}
	return storepostgres.NewRepository(db)
}

func buildUserRepository(db *gorm.DB) userports.Repository {
	if db == nil {
		return usermemory.NewRepository()
	}
	return userpostgres.NewRepository(db)
}

func buildUserSessionStore(db *gorm.DB, sessionTTL time.Duration) userports.SessionStore {
	if db == nil {
		return usermemory.NewSessionStore()
	}
	return userpostgres.NewSessionStore(db, sessionTTL)
}

func connectPostgresWithConfig(ctx context.Context, logger *slog.Logger, dsn string) (*gorm.DB, func()) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		if logger != nil {
			logger.Warn("POSTGRES_DSN not set, falling back to in-memory repositories")
		}
		return nil, func() {}
	}
	db, err := platformpostgres.Connect(ctx, dsn)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to connect to postgres, falling back to in-memory repositories", slog.String("error", err.Error()))
		}
		return nil, func() {}
	}
	sqlDB, err := db.DB()
	if err != nil {
		if logger != nil {
			logger.Warn("failed to unwrap postgres connection, falling back to in-memory repositories", slog.String("error", err.Error()))
		}
		return nil, func() {}
	}
	if logger != nil {
		logger.Info("postgres connection established for repositories")
	}
	return db, func() { _ = sqlDB.Close() }
}

type sessionPurger interface {
	PurgeExpired(ctx context.Context) error
}

// startSessionPurger runs a background ticker to purge expired sessions when configured.
// Controlled by an interval in minutes; when zero, purging is skipped.
func startSessionPurger(ctx context.Context, logger *slog.Logger, store userports.SessionStore, intervalMinutes int) {
	if intervalMinutes <= 0 {
		return
	}
	purger, ok := store.(sessionPurger)
	if !ok {
		if logger != nil {
			logger.Warn("session store does not support purging; skipping session purge")
		}
		return
	}
	interval := time.Duration(intervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)
	if logger != nil {
		logger.Info("session purge enabled", slog.Duration("interval", interval))
	}
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := purger.PurgeExpired(context.Background()); err != nil && logger != nil {
					logger.Warn("session purge failed", slog.String("error", err.Error()))
				}
			}
		}
	}()
}

func registerHealthRoutes(router *gin.Engine, cfg Config, db *gorm.DB, temporalClient client.Client) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/debug/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, debugConfig(cfg))
	})
	router.GET("/readyz", func(c *gin.Context) {
		dbStatus := databaseStatus(c.Request.Context(), db)
		temporalStatus := temporalStatus(c.Request.Context(), temporalClient)
		status := http.StatusOK
		if strings.HasPrefix(dbStatus, "error") || strings.HasPrefix(temporalStatus, "error") {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{
			"status":   "ok",
			"database": dbStatus,
			"temporal": temporalStatus,
		})
	})
}

func databaseStatus(ctx context.Context, db *gorm.DB) string {
	if db == nil {
		return "disabled"
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

func temporalStatus(ctx context.Context, c client.Client) string {
	if c == nil {
		return "inline"
	}
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if _, err := c.WorkflowService().GetSystemInfo(pingCtx, &workflowservice.GetSystemInfoRequest{}); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

func connectTemporalClient(instruments *platformobservability.Instruments, cfg Config) (client.Client, error) {
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
		HostPort:  cfg.TemporalAddress,
		Namespace: effectiveTemporalNamespace(cfg),
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

func effectiveTemporalNamespace(cfg Config) string {
	if ns := strings.TrimSpace(cfg.TemporalNamespace); ns != "" {
		return ns
	}
	return client.DefaultNamespace
}

// debugConfig returns a sanitized view of the runtime config for troubleshooting.
func debugConfig(cfg Config) gin.H {
	return gin.H{
		"port":                        cfg.Port,
		"postgres_enabled":            strings.TrimSpace(cfg.PostgresDSN) != "",
		"temporal_disabled":           cfg.TemporalDisabled,
		"temporal_address_set":        strings.TrimSpace(cfg.TemporalAddress) != "",
		"temporal_namespace":          effectiveTemporalNamespace(cfg),
		"partner_api_enabled":         strings.TrimSpace(cfg.PartnerAPIBaseURL) != "",
		"session_ttl_hours":           cfg.SessionTTL.Hours(),
		"session_purge_interval_mins": cfg.SessionPurgeIntervalMinute,
	}
}
