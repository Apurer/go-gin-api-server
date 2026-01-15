package partner

import (
	"context"
	"errors"

	partnerclient "github.com/GIT_USER_ID/GIT_REPO_ID/internal/clients/http/partner"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

// Syncer implements the outbound partner sync port.
type Syncer struct {
	client *partnerclient.Client
}

// NewSyncer wires a partner HTTP client into a sync adapter.
func NewSyncer(client *partnerclient.Client) *Syncer {
	return &Syncer{client: client}
}

// Sync pushes the pet aggregate to the partner API.
func (s *Syncer) Sync(ctx context.Context, pet *domain.Pet) error {
	if s == nil || s.client == nil {
		return errors.New("partner syncer not configured")
	}
	if pet == nil {
		return errors.New("pet is nil")
	}
	payload := ToPayload(pet)
	return s.client.SyncPet(ctx, payload)
}

var _ ports.PartnerSync = (*Syncer)(nil)
