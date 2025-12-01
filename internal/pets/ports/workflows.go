package ports

import (
	"context"

	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/application/types"
)

// WorkflowOrchestrator exposes durable workflow operations required by the pets bounded context.
type WorkflowOrchestrator interface {
	CreatePet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error)
}
