package ports

import (
	"context"

	"github.com/Apurer/go-gin-api-server/internal/domains/users/domain"
)

// Service exposes user bounded context use cases to adapters.
type Service interface {
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
	CreateUsers(ctx context.Context, users []*domain.User) ([]*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Delete(ctx context.Context, username string) error
	Update(ctx context.Context, username string, updated *domain.User) (*domain.User, error)
	Login(ctx context.Context, username, password string) (string, error)
	Logout(ctx context.Context, username string)
}
