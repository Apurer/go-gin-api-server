package pets

import (
	"context"
	"errors"

	"go.temporal.io/sdk/activity"

	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	petsports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

const (
	// PersistPetActivityName persists a pet aggregate without calling external partners.
	PersistPetActivityName = "pets.activities.PersistPet"
	// SyncPetWithPartnerActivityName triggers partner sync for an existing pet.
	SyncPetWithPartnerActivityName = "pets.activities.SyncPetWithPartner"
)

// Activities groups activities that operate on the pets bounded context.
type Activities struct {
	persistService petsports.Service
	repo           petsports.Repository
	partnerSync    petsports.PartnerSync
}

// NewActivities wires the pets collaborators into the Temporal activities bundle.
// persistService should be constructed without a partner sync dependency to avoid duplicate calls.
func NewActivities(persistService petsports.Service, repo petsports.Repository, partnerSync petsports.PartnerSync) *Activities {
	return &Activities{
		persistService: persistService,
		repo:           repo,
		partnerSync:    partnerSync,
	}
}

// PersistPet stores a new pet aggregate and returns its projection.
func (a *Activities) PersistPet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	logger := activity.GetLogger(ctx)
	petID := input.PetMutationInput.ID
	if a == nil || a.persistService == nil {
		logger.Error("pet persist activity not initialized", "petId", petID)
		return nil, errors.New("pet persist activity not initialized")
	}
	logger.Info("PersistPet activity started", "petId", petID)
	projection, err := a.persistService.AddPet(ctx, input)
	if err != nil {
		logger.Error("PersistPet activity failed", "petId", petID, "error", err)
		return nil, err
	}
	if projection != nil && projection.Pet != nil {
		logger.Info("PersistPet activity completed", "petId", projection.Pet.ID)
	} else {
		logger.Info("PersistPet activity completed")
	}
	return projection, nil
}

// SyncPetWithPartner loads a pet and pushes it to the configured partner.
func (a *Activities) SyncPetWithPartner(ctx context.Context, input petstypes.PetIdentifier) error {
	logger := activity.GetLogger(ctx)
	if a == nil {
		logger.Error("pet sync activity not initialized", "petId", input.ID)
		return errors.New("pet sync activity not initialized")
	}
	if a.partnerSync == nil {
		logger.Info("partner sync not configured; skipping", "petId", input.ID)
		return nil
	}
	if a.repo == nil {
		logger.Error("pet repository not configured for sync", "petId", input.ID)
		return errors.New("pet repository not configured for sync")
	}

	var hb syncHeartbeat
	if activity.HasHeartbeatDetails(ctx) {
		_ = activity.GetHeartbeatDetails(ctx, &hb)
	}
	if hb.Completed {
		logger.Info("SyncPetWithPartner already completed in prior attempt; skipping", "petId", input.ID)
		return nil
	}

	logger.Info("SyncPetWithPartner activity started", "petId", input.ID)
	projection, err := a.repo.GetByID(ctx, input.ID)
	if err != nil {
		logger.Error("SyncPetWithPartner failed to load pet", "petId", input.ID, "error", err)
		return err
	}
	if projection == nil || projection.Pet == nil {
		logger.Error("SyncPetWithPartner missing pet projection", "petId", input.ID)
		return errors.New("pet projection missing for sync")
	}
	if err := a.partnerSync.Sync(ctx, projection.Pet); err != nil {
		logger.Error("SyncPetWithPartner failed", "petId", input.ID, "error", err)
		return err
	}
	activity.RecordHeartbeat(ctx, syncHeartbeat{Completed: true})
	logger.Info("SyncPetWithPartner activity completed", "petId", input.ID)
	return nil
}

type syncHeartbeat struct {
	Completed bool
}
