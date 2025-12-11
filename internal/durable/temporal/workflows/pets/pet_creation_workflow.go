package pets

import (
	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/durable/temporal/sequences"
	"go.temporal.io/sdk/workflow"
)

const (
	// PetCreationWorkflowName is the public identifier for registering the workflow.
	PetCreationWorkflowName = "pets.workflows.Creation"
	// PetCreationTaskQueue is the queue consumed by the worker processing pet workflows.
	PetCreationTaskQueue = "PET_CREATION"
)

// PetCreationWorkflowInput captures the payload required to provision a new pet.
type PetCreationWorkflowInput struct {
	Command petstypes.AddPetInput
	TraceID string
}

// PetCreationWorkflow orchestrates the activities needed to persist a pet aggregate.
func PetCreationWorkflow(ctx workflow.Context, input PetCreationWorkflowInput) (*petstypes.PetProjection, error) {
	logger := workflow.GetLogger(ctx)
	petID := input.Command.PetMutationInput.ID
	logger.Info("PetCreationWorkflow started", withTraceID(input.TraceID, "petId", petID)...)
	projection, err := sequences.RunPetPersistenceSequence(ctx, input.Command)
	if err != nil {
		logger.Error("PetCreationWorkflow failed", withTraceID(input.TraceID, "petId", petID, "error", err)...)
		return nil, err
	}
	if projection != nil && projection.Pet != nil {
		logger.Info("PetCreationWorkflow completed", withTraceID(input.TraceID, "petId", projection.Pet.ID)...)
	} else {
		logger.Info("PetCreationWorkflow completed", withTraceID(input.TraceID)...)
	}
	return projection, nil
}

func withTraceID(traceID string, keyvals ...interface{}) []interface{} {
	if traceID == "" {
		return keyvals
	}
	return append(keyvals, "traceId", traceID)
}
