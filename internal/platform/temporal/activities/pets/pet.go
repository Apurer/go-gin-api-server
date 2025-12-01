package pets

import (
	"context"
	"errors"

	"go.temporal.io/sdk/activity"

	petsservice "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/service"
	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/service/types"
)

const (
	// CreatePetActivityName identifies the Temporal activity that persists a new pet aggregate.
	CreatePetActivityName = "pets.activities.CreatePet"
)

// Activities groups activities that operate on the pets bounded context.
type Activities struct {
	service *petsservice.Service
}

// NewActivities wires the pets service into the Temporal activities bundle.
func NewActivities(service *petsservice.Service) *Activities {
	return &Activities{service: service}
}

// CreatePet persists a new pet aggregate via the domain service.
func (a *Activities) CreatePet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	logger := activity.GetLogger(ctx)
	petID := input.PetMutationInput.ID
	if a == nil || a.service == nil {
		logger.Error("pet activities not initialized", "petId", petID)
		return nil, errors.New("pet activities not initialized")
	}
	logger.Info("CreatePet activity started", "petId", petID)
	projection, err := a.service.AddPet(ctx, input)
	if err != nil {
		logger.Error("CreatePet activity failed", "petId", petID, "error", err)
		return nil, err
	}
	if projection != nil && projection.Pet != nil {
		logger.Info("CreatePet activity completed", "petId", projection.Pet.ID)
	} else {
		logger.Info("CreatePet activity completed")
	}
	return projection, nil
}
