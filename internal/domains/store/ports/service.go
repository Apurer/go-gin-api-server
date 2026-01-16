package ports

import (
	"context"

	"github.com/Apurer/go-gin-api-server/internal/domains/store/domain"
)

// Service exposes store/order use cases to adapters.
type Service interface {
	PlaceOrder(ctx context.Context, order *domain.Order) (*domain.Order, error)
	GetOrderByID(ctx context.Context, id int64) (*domain.Order, error)
	DeleteOrder(ctx context.Context, id int64) error
	Inventory(ctx context.Context) (map[string]int32, error)
}
