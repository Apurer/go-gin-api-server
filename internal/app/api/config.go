package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.temporal.io/sdk/client"
)

// Config carries environment-driven settings for the API process.
type Config struct {
	Port                       string
	PostgresDSN                string
	TemporalAddress            string
	TemporalNamespace          string
	TemporalDisabled           bool
	SessionPurgeIntervalMinute int
}

// LoadConfig reads environment variables, applies defaults, and validates basic constraints.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:              envDefault("PORT", "8080"),
		PostgresDSN:       strings.TrimSpace(os.Getenv("POSTGRES_DSN")),
		TemporalAddress:   envDefault("TEMPORAL_ADDRESS", client.DefaultHostPort),
		TemporalNamespace: envDefault("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		TemporalDisabled:  isTruthy(os.Getenv("TEMPORAL_DISABLED")),
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_PURGE_INTERVAL_MINUTES")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err != nil || minutes <= 0 {
			return Config{}, fmt.Errorf("SESSION_PURGE_INTERVAL_MINUTES must be a positive integer")
		}
		cfg.SessionPurgeIntervalMinute = minutes
	}
	return cfg, nil
}

func envDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func isTruthy(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
}
