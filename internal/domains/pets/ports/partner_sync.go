package ports

import (
	"context"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
)

// PartnerSync defines outbound integration for syncing pets with an external provider.
type PartnerSync interface {
	Sync(ctx context.Context, pet *domain.Pet) error
}
