package ports

import (
	"context"
	"errors"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"
)

var ErrNotFound = errors.New("user not found")
var ErrInvalidCredentials = errors.New("invalid username or password")

type Repository interface {
	Save(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Delete(ctx context.Context, username string) error
	List(ctx context.Context) ([]*domain.User, error)
}
