package memory

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/ports"
)

var _ ports.Repository = (*Repository)(nil)

// Repository is an in-memory user persistence adapter.
type Repository struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func NewRepository() *Repository {
	return &Repository{users: map[string]*domain.User{}}
}

func normalize(username string) string {
	return strings.ToLower(username)
}

func (r *Repository) Save(_ context.Context, user *domain.User) (*domain.User, error) {
	if user.Username == "" {
		return nil, errors.New("username is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	clone := *user
	r.users[normalize(user.Username)] = &clone
	return &clone, nil
}

func (r *Repository) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[normalize(username)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	clone := *user
	return &clone, nil
}

func (r *Repository) Delete(_ context.Context, username string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[normalize(username)]; !ok {
		return ports.ErrNotFound
	}
	delete(r.users, normalize(username))
	return nil
}

func (r *Repository) List(_ context.Context) ([]*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		clone := *user
		list = append(list, &clone)
	}
	return list, nil
}
