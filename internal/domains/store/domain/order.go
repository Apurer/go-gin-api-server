package domain

import (
	"errors"
	"time"
)

// Status enumerates order progression.
type Status string

const (
	StatusPlaced    Status = "placed"
	StatusApproved  Status = "approved"
	StatusDelivered Status = "delivered"
)

var (
	ErrInvalidPetID   = errors.New("pet id must be greater than zero")
	ErrInvalidQuantity = errors.New("quantity must be greater than zero")
	ErrInvalidStatus  = errors.New("order status is invalid")
)

// Order models the store purchase order aggregate.
type Order struct {
	ID       int64
	PetID    int64
	Quantity int32
	ShipDate time.Time
	Status   Status
	Complete bool
}

// NewOrder validates and constructs a new Order aggregate.
func NewOrder(id int64, petID int64, quantity int32, shipDate time.Time, status Status, complete bool) (*Order, error) {
	order := &Order{
		ID:       id,
		PetID:    petID,
		Quantity: quantity,
		ShipDate: shipDate,
		Status:   status,
		Complete: complete,
	}
	if err := order.UpdateStatus(order.Status); err != nil {
		return nil, err
	}
	if err := order.Validate(); err != nil {
		return nil, err
	}
	return order, nil
}

// Validate enforces invariants on the aggregate.
func (o *Order) Validate() error {
	if o.PetID <= 0 {
		return ErrInvalidPetID
	}
	if o.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if !isValidStatus(o.Status) {
		return ErrInvalidStatus
	}
	return nil
}

// UpdateStatus ensures only known states are accepted and defaults to placed.
func (o *Order) UpdateStatus(status Status) error {
	if status == "" {
		status = StatusPlaced
	}
	if !isValidStatus(status) {
		return ErrInvalidStatus
	}
	o.Status = status
	return nil
}

func isValidStatus(status Status) bool {
	switch status {
	case StatusPlaced, StatusApproved, StatusDelivered:
		return true
	default:
		return false
	}
}
