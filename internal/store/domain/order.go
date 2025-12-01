package domain

import "time"

// Status enumerates order progression.
type Status string

const (
	StatusPlaced    Status = "placed"
	StatusApproved  Status = "approved"
	StatusDelivered Status = "delivered"
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
