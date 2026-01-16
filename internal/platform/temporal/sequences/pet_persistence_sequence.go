package sequences

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	petstypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	petactivities "github.com/Apurer/go-gin-api-server/internal/platform/temporal/activities/pets"
)

// RunPetPersistenceSequence executes the ordered set of activities needed to persist a pet aggregate.
func RunPetPersistenceSequence(ctx workflow.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	logger := workflow.GetLogger(ctx)
	petID := input.PetMutationInput.ID
	logger.Info("pet persistence sequence started", "petId", petID)
	persistOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    5,
		},
	}
	syncOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	}

	var projection petstypes.PetProjection
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, persistOptions), petactivities.PersistPetActivityName, input).Get(ctx, &projection)
	if err != nil {
		logger.Error("pet persistence sequence failed", "petId", petID, "error", err)
		return nil, err
	}
	if projection.Pet != nil {
		logger.Info("pet persistence sequence persisted", "petId", projection.Pet.ID)
	} else {
		logger.Info("pet persistence sequence persisted")
	}

	// Sync to partner with separate retry policy.
	if projection.Pet != nil {
		syncInput := petstypes.PetIdentifier{ID: projection.Pet.ID}
		if err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, syncOptions), petactivities.SyncPetWithPartnerActivityName, syncInput).Get(ctx, nil); err != nil {
			logger.Error("pet persistence sequence sync failed", "petId", petID, "error", err)
			return &projection, err
		}
		logger.Info("pet persistence sequence synced", "petId", projection.Pet.ID)
	}
	return &projection, nil
}
