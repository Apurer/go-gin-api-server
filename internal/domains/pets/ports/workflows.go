package ports

import (
	"context"

	petstypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
)

// WorkflowOrchestrator exposes durable workflow operations required by the pets bounded context.
type WorkflowOrchestrator interface {
	CreatePet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error)
}
