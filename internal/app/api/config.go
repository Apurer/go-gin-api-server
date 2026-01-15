package api

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk/client"
)

const defaultSessionTTLHours = 24

// Config carries environment-driven settings for the API process.
type Config struct {
	Port                       string
	PostgresDSN                string
	TemporalAddress            string
	TemporalNamespace          string
	TemporalDisabled           bool
	SessionPurgeIntervalMinute int
	SessionTTL                 time.Duration
}

// LoadConfig reads environment variables, applies defaults, and validates basic constraints.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:              envDefault("PORT", "8080"),
		PostgresDSN:       strings.TrimSpace(os.Getenv("POSTGRES_DSN")),
		TemporalAddress:   envDefault("TEMPORAL_ADDRESS", client.DefaultHostPort),
		TemporalNamespace: envDefault("TEMPORAL_NAMESPACE", client.DefaultNamespace),
		TemporalDisabled:  isTruthy(os.Getenv("TEMPORAL_DISABLED")),
		SessionTTL:        time.Duration(defaultSessionTTLHours) * time.Hour,
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_PURGE_INTERVAL_MINUTES")); raw != "" {
		minutes, err := strconv.Atoi(raw)
		if err != nil || minutes <= 0 {
			return Config{}, fmt.Errorf("SESSION_PURGE_INTERVAL_MINUTES must be a positive integer")
		}
		cfg.SessionPurgeIntervalMinute = minutes
	}
	if raw := strings.TrimSpace(os.Getenv("SESSION_TTL_HOURS")); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours <= 0 {
			return Config{}, fmt.Errorf("SESSION_TTL_HOURS must be a positive integer")
		}
		cfg.SessionTTL = time.Duration(hours) * time.Hour
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
