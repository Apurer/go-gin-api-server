package ports

import (
	"context"
	"errors"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/shared/projection"
)

var ErrNotFound = errors.New("pet not found")

type Repository interface {
	Save(ctx context.Context, pet *domain.Pet) (*projection.Projection[*domain.Pet], error)
	GetByID(ctx context.Context, id int64) (*projection.Projection[*domain.Pet], error)
	Delete(ctx context.Context, id int64) error
	FindByStatus(ctx context.Context, statuses []domain.Status) ([]*projection.Projection[*domain.Pet], error)
	FindByTags(ctx context.Context, tags []string) ([]*projection.Projection[*domain.Pet], error)
	List(ctx context.Context) ([]*projection.Projection[*domain.Pet], error)
}
