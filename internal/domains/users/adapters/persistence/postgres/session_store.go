package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
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

const defaultSessionTTL = 24 * time.Hour
const sessionTTLEnv = "SESSION_TTL_HOURS"

// NewSessionStore wires a PostgreSQL-backed session store. Caller owns DB lifecycle.
func NewSessionStore(db *gorm.DB) *SessionStore {
	store := &SessionStore{db: db}
	return store
}

type sessionRecord struct {
	Token     string     `gorm:"primaryKey;column:token;size:512"`
	Username  string     `gorm:"column:username;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at;index"`
	CreatedAt time.Time  `gorm:"column:created_at;index"`
	UpdatedAt time.Time  `gorm:"column:updated_at;index"`
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
	expiry := time.Now().Add(sessionTTLFromEnv())
	rec := sessionRecord{Username: username, Token: token, ExpiresAt: &expiry}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token"}},
			DoUpdates: clause.AssignmentColumns([]string{"username", "expires_at", "updated_at"}),
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

// PurgeExpired removes all expired sessions. Use for housekeeping or cron.
func (s *SessionStore) PurgeExpired(ctx context.Context) error {
	if err := s.ensureDB(); err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Where("expires_at IS NOT NULL AND expires_at <= ?", now).Delete(&sessionRecord{}).Error
}

func (s *SessionStore) ensureDB() error {
	if s == nil || s.db == nil {
		return errors.New("postgres session store not configured")
	}
	return nil
}

func sessionTTLFromEnv() time.Duration {
	if raw := strings.TrimSpace(os.Getenv(sessionTTLEnv)); raw != "" {
		if hours, err := strconv.Atoi(raw); err == nil && hours > 0 {
			return time.Duration(hours) * time.Hour
		}
	}
	return defaultSessionTTL
}

var _ userports.SessionStore = (*SessionStore)(nil)
