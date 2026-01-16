package ports

import (
	"context"

	pettypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
)

// UploadImageResult describes the metadata returned by the upload flow.
type UploadImageResult struct {
	Code    int32
	Type    string
	Message string
}

// Service defines the pets use cases exposed to adapters (inbound/driving port).
type Service interface {
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
