package pets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"

	"go.temporal.io/sdk/activity"

	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	petsports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

const (
	// PersistPetActivityName persists a pet aggregate without calling external partners.
	PersistPetActivityName = "pets.activities.PersistPet"
	// SyncPetWithPartnerActivityName triggers partner sync for an existing pet.
	SyncPetWithPartnerActivityName = "pets.activities.SyncPetWithPartner"

	partnerSyncHashKey = "partner_sync_hash"
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
	hash, err := computePartnerSyncHash(projection.Pet)
	if err != nil {
		logger.Error("SyncPetWithPartner failed to compute hash", "petId", input.ID, "error", err)
		return err
	}
	if alreadySynced(hash, projection.Pet) {
		logger.Info("SyncPetWithPartner skipped; payload unchanged since last sync", "petId", input.ID)
		return nil
	}
	if err := a.partnerSync.Sync(ctx, projection.Pet); err != nil {
		logger.Error("SyncPetWithPartner failed", "petId", input.ID, "error", err)
		return err
	}
	// Persist the sync hash to avoid re-sending identical payloads on retries.
	updatePartnerSyncHash(hash, projection.Pet)
	if _, err := a.repo.Save(ctx, projection.Pet); err != nil {
		logger.Error("SyncPetWithPartner failed to persist sync hash", "petId", input.ID, "error", err)
		return err
	}
	activity.RecordHeartbeat(ctx, syncHeartbeat{Completed: true, Hash: hash})
	logger.Info("SyncPetWithPartner activity completed", "petId", input.ID)
	return nil
}

type syncHeartbeat struct {
	Completed bool
	Hash      string
}

// computePartnerSyncHash builds a deterministic hash of the pet payload used for partner sync.
func computePartnerSyncHash(p *domain.Pet) (string, error) {
	if p == nil {
		return "", errors.New("nil pet")
	}
	normalized := struct {
		ID           int64             `json:"id"`
		Name         string            `json:"name"`
		Status       domain.Status     `json:"status"`
		CategoryID   int64             `json:"categoryId"`
		CategoryName string            `json:"categoryName"`
		PhotoURLs    []string          `json:"photoUrls"`
		Tags         []syncTag         `json:"tags"`
		Attributes   []syncAttributeKV `json:"attributes"`
	}{
		ID:        p.ID,
		Name:      p.Name,
		Status:    p.Status,
		PhotoURLs: append([]string{}, p.PhotoURLs...),
	}
	if p.Category != nil {
		normalized.CategoryID = p.Category.ID
		normalized.CategoryName = p.Category.Name
	}
	if len(p.Tags) > 0 {
		tags := make([]syncTag, 0, len(p.Tags))
		for _, t := range p.Tags {
			tags = append(tags, syncTag{ID: t.ID, Name: t.Name})
		}
		sort.Slice(tags, func(i, j int) bool {
			if tags[i].Name == tags[j].Name {
				return tags[i].ID < tags[j].ID
			}
			return tags[i].Name < tags[j].Name
		})
		normalized.Tags = tags
	}
	if p.ExternalRef != nil && len(p.ExternalRef.Attributes) > 0 {
		attrs := make([]syncAttributeKV, 0, len(p.ExternalRef.Attributes))
		for k, v := range p.ExternalRef.Attributes {
			attrs = append(attrs, syncAttributeKV{Key: k, Value: v})
		}
		sort.Slice(attrs, func(i, j int) bool { return attrs[i].Key < attrs[j].Key })
		normalized.Attributes = attrs
	}

	bytes, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:]), nil
}

func alreadySynced(hash string, p *domain.Pet) bool {
	if p == nil || p.ExternalRef == nil {
		return false
	}
	return p.ExternalRef.Attributes[partnerSyncHashKey] == hash
}

func updatePartnerSyncHash(hash string, p *domain.Pet) {
	if p.ExternalRef == nil {
		p.UpdateExternalReference(&domain.ExternalReference{
			Provider: "partner",
			ID:       strconv.FormatInt(p.ID, 10),
		})
	}
	if p.ExternalRef.Attributes == nil {
		p.ExternalRef.Attributes = map[string]string{}
	}
	p.ExternalRef.Attributes[partnerSyncHashKey] = hash
}

type syncTag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type syncAttributeKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
