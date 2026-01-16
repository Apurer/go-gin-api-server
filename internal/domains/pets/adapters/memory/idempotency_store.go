package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
)

var _ ports.IdempotencyStore = (*IdempotencyStore)(nil)

// IdempotencyStore provides an in-memory implementation for development and tests.
type IdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]ports.IdempotencyRecord
	now     func() time.Time
}

// NewIdempotencyStore constructs an empty in-memory store.
func NewIdempotencyStore() *IdempotencyStore {
	return &IdempotencyStore{
		records: map[string]ports.IdempotencyRecord{},
		now:     time.Now,
	}
}

// WithClock overrides the time source for deterministic testing.
func (s *IdempotencyStore) WithClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// Get returns the stored record for the provided key, or nil when absent.
func (s *IdempotencyStore) Get(_ context.Context, key string) (*ports.IdempotencyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.records[key]
	if !ok {
		return nil, nil
	}
	copy := record
	return &copy, nil
}

// Save persists the record or returns the existing record if it matches.
func (s *IdempotencyStore) Save(_ context.Context, record ports.IdempotencyRecord) (*ports.IdempotencyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.records[record.Key]; ok {
		if existing.RequestHash != record.RequestHash || existing.PetID != record.PetID {
			copy := existing
			return &copy, ports.ErrIdempotencyConflict
		}
		copy := existing
		return &copy, nil
	}

	now := s.now()
	record.CreatedAt = now
	record.UpdatedAt = now
	s.records[record.Key] = record
	saved := record
	return &saved, nil
}
