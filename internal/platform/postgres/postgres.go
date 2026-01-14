package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect opens a PostgreSQL connection via GORM and verifies connectivity.
func Connect(ctx context.Context, dsn string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres DSN is empty")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// ConnectFromEnv dials PostgreSQL using POSTGRES_DSN and returns the DB plus a cleanup function.
// When POSTGRES_DSN is missing or the connection fails, it logs and returns nil with a no-op cleanup.
func ConnectFromEnv(ctx context.Context, logger *slog.Logger) (*gorm.DB, func()) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		if logger != nil {
			logger.Warn("POSTGRES_DSN not set, falling back to in-memory repositories")
		}
		return nil, func() {}
	}
	db, err := Connect(ctx, dsn)
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
		logger.Info("postgres connection established")
	}
	return db, func() { _ = sqlDB.Close() }
}
