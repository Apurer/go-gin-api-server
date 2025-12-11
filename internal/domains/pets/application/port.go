package application

import (
	"context"

	pettypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
)

// Port defines the pets use cases exposed to adapters.
type Port interface {
	AddPet(ctx context.Context, input pettypes.AddPetInput) (*pettypes.PetProjection, error)
	UpdatePet(ctx context.Context, input pettypes.UpdatePetInput) (*pettypes.PetProjection, error)
	UpdatePetWithForm(ctx context.Context, input pettypes.UpdatePetWithFormInput) (*pettypes.PetProjection, error)
	FindByStatus(ctx context.Context, input pettypes.FindPetsByStatusInput) ([]*pettypes.PetProjection, error)
	FindByTags(ctx context.Context, input pettypes.FindPetsByTagsInput) ([]*pettypes.PetProjection, error)
	GetByID(ctx context.Context, input pettypes.PetIdentifier) (*pettypes.PetProjection, error)
	Delete(ctx context.Context, input pettypes.PetIdentifier) error
	GroomPet(ctx context.Context, input pettypes.GroomPetInput) (*pettypes.PetProjection, error)
	UploadImage(ctx context.Context, input pettypes.UploadImageInput) (*UploadImageResult, error)
	List(ctx context.Context) ([]*pettypes.PetProjection, error)
}
