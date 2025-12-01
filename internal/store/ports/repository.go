package ports

import (
	"context"
	"errors"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/store/domain"
)

var ErrNotFound = errors.New("order not found")

// Repository persists orders and exposes inventory views.
type Repository interface {
	Save(ctx context.Context, order *domain.Order) (*domain.Order, error)
	GetByID(ctx context.Context, id int64) (*domain.Order, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*domain.Order, error)
}
