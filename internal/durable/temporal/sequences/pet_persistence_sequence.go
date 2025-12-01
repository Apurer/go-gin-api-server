package sequences

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	petactivities "github.com/GIT_USER_ID/GIT_REPO_ID/internal/durable/temporal/activities/pets"
	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/application/types"
)

// RunPetPersistenceSequence executes the ordered set of activities needed to persist a pet aggregate.
func RunPetPersistenceSequence(ctx workflow.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	logger := workflow.GetLogger(ctx)
	petID := input.PetMutationInput.ID
	logger.Info("pet persistence sequence started", "petId", petID)
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var projection petstypes.PetProjection
	err := workflow.ExecuteActivity(ctx, petactivities.CreatePetActivityName, input).Get(ctx, &projection)
	if err != nil {
		logger.Error("pet persistence sequence failed", "petId", petID, "error", err)
		return nil, err
	}
	if projection.Pet != nil {
		logger.Info("pet persistence sequence completed", "petId", projection.Pet.ID)
	} else {
		logger.Info("pet persistence sequence completed")
	}
	return &projection, nil
}
