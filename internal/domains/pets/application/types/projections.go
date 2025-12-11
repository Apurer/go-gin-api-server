package types

import (
	"time"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/shared/projection"
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

// FromDomainProjection adapts a shared projection into the application-friendly DTO.
func FromDomainProjection(source *projection.Projection[*domain.Pet]) *PetProjection {
	if source == nil {
		return nil
	}
	return &PetProjection{
		Pet: source.Entity,
		Metadata: PetMetadata{
			CreatedAt: source.Metadata.CreatedAt,
			UpdatedAt: source.Metadata.UpdatedAt,
		},
	}
}

// FromDomainProjectionList adapts a slice of shared projections.
func FromDomainProjectionList(sources []*projection.Projection[*domain.Pet]) []*PetProjection {
	if len(sources) == 0 {
		return nil
	}
	result := make([]*PetProjection, 0, len(sources))
	for _, src := range sources {
		if converted := FromDomainProjection(src); converted != nil {
			result = append(result, converted)
		}
	}
	return result
}
