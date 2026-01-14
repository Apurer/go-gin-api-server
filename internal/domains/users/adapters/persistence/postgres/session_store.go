package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	userports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/ports"
)

// SessionStore persists user sessions in PostgreSQL.
type SessionStore struct {
	db *gorm.DB
}

// NewSessionStore wires a PostgreSQL-backed session store. Caller owns DB lifecycle.
func NewSessionStore(db *gorm.DB) *SessionStore {
	store := &SessionStore{db: db}
	if db != nil {
		_ = db.AutoMigrate(&sessionRecord{})
	}
	return store
}

type sessionRecord struct {
	Username  string    `gorm:"primaryKey;column:username"`
	Token     string    `gorm:"column:token"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (sessionRecord) TableName() string { return "user_sessions" }

// Save upserts a session token keyed by username.
func (s *SessionStore) Save(ctx context.Context, username, token string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	token = strings.TrimSpace(token)
	if username == "" || token == "" {
		return errors.New("username and token are required")
	}
	rec := sessionRecord{Username: username, Token: token}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "username"}},
			DoUpdates: clause.AssignmentColumns([]string{"token", "updated_at"}),
		}).
		Create(&rec).Error
}

// Delete removes a session by username.
func (s *SessionStore) Delete(ctx context.Context, username string) error {
	if err := s.ensureDB(); err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	return s.db.WithContext(ctx).Delete(&sessionRecord{}, "username = ?", username).Error
}

func (s *SessionStore) ensureDB() error {
	if s == nil || s.db == nil {
		return errors.New("postgres session store not configured")
	}
	return nil
}

var _ userports.SessionStore = (*SessionStore)(nil)
