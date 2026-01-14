package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/ports"
)

var _ ports.Repository = (*Repository)(nil)

// Repository persists users in PostgreSQL using GORM.
type Repository struct {
	db *gorm.DB
}

// NewRepository wires a PostgreSQL-backed repository. Caller manages DB lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	repo := &Repository{db: db}
	if db != nil {
		_ = db.AutoMigrate(&userRecord{})
	}
	return repo
}

type userRecord struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Username  string    `gorm:"column:username;uniqueIndex"`
	FirstName string    `gorm:"column:first_name"`
	LastName  string    `gorm:"column:last_name"`
	Email     string    `gorm:"column:email"`
	Password  string    `gorm:"column:password_hash"`
	Phone     string    `gorm:"column:phone"`
	Status    int32     `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (userRecord) TableName() string { return "users" }

// Save inserts or updates a user keyed by username.
func (r *Repository) Save(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user is nil")
	}
	clone := *user
	if err := clone.Validate(); err != nil {
		return nil, err
	}
	record := toRecord(&clone)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}},
			DoUpdates: clause.AssignmentColumns([]string{"first_name", "last_name", "email", "password_hash", "phone", "status", "updated_at"}),
		}).
		Create(&record).Error; err != nil {
		return nil, err
	}
	return r.GetByUsername(ctx, record.Username)
}

// GetByUsername fetches a user by username.
func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	var record userRecord
	if err := r.db.WithContext(ctx).First(&record, "username = ?", username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return record.toDomain(), nil
}

// Delete removes a user by username.
func (r *Repository) Delete(ctx context.Context, username string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	result := r.db.WithContext(ctx).Where("username = ?", username).Delete(&userRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// List returns all users.
func (r *Repository) List(ctx context.Context) ([]*domain.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []userRecord
	if err := r.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	users := make([]*domain.User, 0, len(records))
	for i := range records {
		users = append(users, records[i].toDomain())
	}
	return users, nil
}

func (r *Repository) ensureDB() error {
	if r == nil || r.db == nil {
		return errors.New("postgres user repository not configured")
	}
	return nil
}

func toRecord(user *domain.User) userRecord {
	return userRecord{
		ID:        user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Password:  user.Password,
		Phone:     user.Phone,
		Status:    user.Status,
	}
}

func (r userRecord) toDomain() *domain.User {
	return &domain.User{
		ID:        r.ID,
		Username:  r.Username,
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
		Password:  r.Password,
		Phone:     r.Phone,
		Status:    r.Status,
	}
}
