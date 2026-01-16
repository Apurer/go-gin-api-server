package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	userpostgres "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/persistence/postgres"
	platformpostgres "github.com/Apurer/go-gin-api-server/internal/platform/postgres"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	db, cleanup := platformpostgres.ConnectFromEnv(ctx, logger)
	defer cleanup()
	if db == nil {
		log.Fatal("POSTGRES_DSN not set or connection failed; cannot purge sessions")
	}

	store := userpostgres.NewSessionStore(db, sessionTTLFromEnv())
	if err := store.PurgeExpired(ctx); err != nil {
		log.Fatalf("failed to purge sessions: %v", err)
	}
	log.Printf("session purge completed")
}

func sessionTTLFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SESSION_TTL_HOURS"))
	if raw == "" {
		return userpostgres.DefaultSessionTTL
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return userpostgres.DefaultSessionTTL
	}
	return time.Duration(hours) * time.Hour
}
