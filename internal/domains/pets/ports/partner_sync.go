package ports

import (
	"context"

	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
)

// PartnerSync defines outbound integration for syncing pets with an external provider.
type PartnerSync interface {
	Sync(ctx context.Context, pet *domain.Pet) error
}
