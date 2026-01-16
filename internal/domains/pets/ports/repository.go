package ports

import (
	"context"
	"errors"

	pettypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

var ErrNotFound = errors.New("pet not found")

type Repository interface {
	Save(ctx context.Context, pet *domain.Pet) (*pettypes.PetProjection, error)
	GetByID(ctx context.Context, id int64) (*pettypes.PetProjection, error)
	Delete(ctx context.Context, id int64) error
	FindByStatus(ctx context.Context, statuses []domain.Status) ([]*pettypes.PetProjection, error)
	FindByTags(ctx context.Context, tags []string) ([]*pettypes.PetProjection, error)
	List(ctx context.Context) ([]*pettypes.PetProjection, error)
}
