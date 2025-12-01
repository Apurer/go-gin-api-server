package http

import (
	"time"

	storedomain "github.com/GIT_USER_ID/GIT_REPO_ID/internal/store/domain"
)

// Order represents the transport-layer shape used by the generated handlers.
type Order struct {
	ID       int64
	PetID    int64
	Quantity int32
	ShipDate time.Time
	Status   string
	Complete bool
}

// ToDomainOrder converts a transport order into the store domain model.
func ToDomainOrder(order Order) *storedomain.Order {
	return &storedomain.Order{
		ID:       order.ID,
		PetID:    order.PetID,
		Quantity: order.Quantity,
		ShipDate: order.ShipDate,
		Status:   storedomain.Status(order.Status),
		Complete: order.Complete,
	}
}

// FromDomainOrder converts a domain order to the transport representation.
func FromDomainOrder(order *storedomain.Order) Order {
	if order == nil {
		return Order{}
	}
	return Order{
		ID:       order.ID,
		PetID:    order.PetID,
		Quantity: order.Quantity,
		ShipDate: order.ShipDate,
		Status:   string(order.Status),
		Complete: order.Complete,
	}
}
