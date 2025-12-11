package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/ports"
)

// Service exposes user bounded context use cases.
type Service struct {
	repo     ports.Repository
	sessions ports.SessionStore
}

func NewService(repo ports.Repository, sessions ports.SessionStore) *Service {
	if sessions == nil {
		sessions = ports.NoopSessionStore
	}
	return &Service{repo: repo, sessions: sessions}
}

func (s *Service) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if user == nil {
		return nil, errors.New("user is nil")
	}
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Save(ctx, user)
}

func (s *Service) CreateUsers(ctx context.Context, users []*domain.User) ([]*domain.User, error) {
	var saved []*domain.User
	for _, u := range users {
		if u == nil {
			return nil, errors.New("user is nil")
		}
		if err := u.Validate(); err != nil {
			return nil, err
		}
		persisted, err := s.repo.Save(ctx, u)
		if err != nil {
			return nil, err
		}
		saved = append(saved, persisted)
	}
	return saved, nil
}

func (s *Service) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *Service) Delete(ctx context.Context, username string) error {
	_ = s.sessions.Delete(ctx, username)
	return s.repo.Delete(ctx, username)
}

func (s *Service) Update(ctx context.Context, username string, updated *domain.User) (*domain.User, error) {
	if updated == nil {
		return nil, errors.New("user is nil")
	}
	existing, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	updated.ID = existing.ID
	if err := updated.SetUsername(username); err != nil {
		return nil, err
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Save(ctx, updated)
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return "", ports.ErrInvalidCredentials
	}
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if !user.CheckPassword(password) {
		return "", ports.ErrInvalidCredentials
	}
	token := fmt.Sprintf("%s:%d", username, time.Now().UnixNano())
	if err := s.sessions.Save(ctx, username, token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) Logout(ctx context.Context, username string) {
	if strings.TrimSpace(username) == "" {
		return
	}
	_ = s.sessions.Delete(ctx, username)
}

var _ ports.Service = (*Service)(nil)
