package types

import (
	"time"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

// PetMetadata captures infrastructure timestamps associated with a persisted pet.
type PetMetadata struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PetProjection transports a domain aggregate together with its persistence metadata.
type PetProjection struct {
	Pet      *domain.Pet
	Metadata PetMetadata
}

// NewPetProjection wraps an aggregate with persistence metadata.
func NewPetProjection(pet *domain.Pet, createdAt, updatedAt time.Time) *PetProjection {
	if pet == nil {
		return nil
	}
	return &PetProjection{
		Pet: pet,
		Metadata: PetMetadata{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}
}

// CloneProjectionList duplicates a slice of projections.
func CloneProjectionList(sources []*PetProjection) []*PetProjection {
	if len(sources) == 0 {
		return nil
	}
	result := make([]*PetProjection, 0, len(sources))
	for _, src := range sources {
		if src == nil {
			continue
		}
		clone := *src
		result = append(result, &clone)
	}
	return result
}
