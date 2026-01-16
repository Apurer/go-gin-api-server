package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
)

var _ ports.IdempotencyStore = (*IdempotencyStore)(nil)

// IdempotencyStore persists idempotency keys in PostgreSQL.
type IdempotencyStore struct {
	db *gorm.DB
}

// NewIdempotencyStore wires a PostgreSQL-backed idempotency store.
func NewIdempotencyStore(db *gorm.DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// Get loads a record by key, returning nil when absent.
func (s *IdempotencyStore) Get(ctx context.Context, key string) (*ports.IdempotencyRecord, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	var record idempotencyRecord
	if err := s.db.WithContext(ctx).First(&record, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toPortRecord(&record), nil
}

// Save inserts the record; if the key already exists with the same hash/pet it is returned,
// otherwise ErrIdempotencyConflict is returned with the stored record.
func (s *IdempotencyStore) Save(ctx context.Context, record ports.IdempotencyRecord) (*ports.IdempotencyRecord, error) {
	if err := s.ensureDB(); err != nil {
		return nil, err
	}
	dbRecord := toDBRecord(record)
	if err := s.db.WithContext(ctx).Create(&dbRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			existing, err := s.Get(ctx, record.Key)
			if err != nil {
				return nil, err
			}
			if existing == nil {
				return nil, err
			}
			if existing.RequestHash != record.RequestHash || existing.PetID != record.PetID {
				return existing, ports.ErrIdempotencyConflict
			}
			return existing, nil
		}
		return nil, err
	}
	return toPortRecord(&dbRecord), nil
}

func (s *IdempotencyStore) ensureDB() error {
	if s == nil || s.db == nil {
		return errors.New("postgres idempotency store not configured")
	}
	return nil
}

type idempotencyRecord struct {
	Key         string    `gorm:"primaryKey;column:key;size:255"`
	RequestHash string    `gorm:"column:request_hash;size:128"`
	PetID       int64     `gorm:"column:pet_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (idempotencyRecord) TableName() string { return "pet_idempotency_keys" }

func toDBRecord(rec ports.IdempotencyRecord) idempotencyRecord {
	return idempotencyRecord{
		Key:         rec.Key,
		RequestHash: rec.RequestHash,
		PetID:       rec.PetID,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

func toPortRecord(rec *idempotencyRecord) *ports.IdempotencyRecord {
	if rec == nil {
		return nil
	}
	return &ports.IdempotencyRecord{
		Key:         rec.Key,
		RequestHash: rec.RequestHash,
		PetID:       rec.PetID,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}
