package ports

import (
	"context"
	"errors"
	"time"
)

// ErrIdempotencyConflict indicates the same key was used with a different payload or target.
var ErrIdempotencyConflict = errors.New("idempotency conflict")

// IdempotencyRecord captures the association between a client-supplied key and the resulting pet.
type IdempotencyRecord struct {
	Key         string
	RequestHash string
	PetID       int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IdempotencyStore persists idempotency keys so retries can be replayed safely.
type IdempotencyStore interface {
	// Get returns the stored record for the key, or nil when unknown.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)
	// Save persists the record; if the key already exists with the same hash and pet, the stored record is returned.
	// When the key exists but points to a different request/pet, ErrIdempotencyConflict is returned with the stored record.
	Save(ctx context.Context, record IdempotencyRecord) (*IdempotencyRecord, error)
}
