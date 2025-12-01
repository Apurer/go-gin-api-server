package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/ports"
)

// Service exposes user bounded context use cases.
type Service struct {
	repo    ports.Repository
	session sync.Map // username -> token
}

func NewService(repo ports.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	return s.repo.Save(ctx, user)
}

func (s *Service) CreateUsers(ctx context.Context, users []*domain.User) ([]*domain.User, error) {
	var saved []*domain.User
	for _, u := range users {
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
	s.session.Delete(username)
	return s.repo.Delete(ctx, username)
}

func (s *Service) Update(ctx context.Context, username string, updated *domain.User) (*domain.User, error) {
	_, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	updated.Username = username
	return s.repo.Save(ctx, updated)
}

func (s *Service) Login(_ context.Context, username, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("username and password are required")
	}
	token := fmt.Sprintf("%s:%s", username, password)
	s.session.Store(username, token)
	return token, nil
}

func (s *Service) Logout(_ context.Context, username string) {
	s.session.Delete(username)
}
