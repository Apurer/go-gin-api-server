package domain

import "time"

// Event is the base interface for all domain events.
type Event interface {
	EventName() string
	OccurredAt() time.Time
}

// BaseEvent provides common event metadata.
type BaseEvent struct {
	Timestamp time.Time
}

// OccurredAt returns when the event occurred.
func (e BaseEvent) OccurredAt() time.Time {
	return e.Timestamp
}

// PetCreated is raised when a new pet is added to the store.
type PetCreated struct {
	BaseEvent
	PetID    int64
	Name     string
	Status   Status
	Category *Category
}

// EventName returns the event type identifier.
func (e PetCreated) EventName() string {
	return "pets.pet.created"
}

// PetUpdated is raised when a pet's attributes are modified.
type PetUpdated struct {
	BaseEvent
	PetID          int64
	Name           string
	Status         Status
	PreviousStatus Status
}

// EventName returns the event type identifier.
func (e PetUpdated) EventName() string {
	return "pets.pet.updated"
}

// PetGroomed is raised when a grooming operation is performed.
type PetGroomed struct {
	BaseEvent
	PetID            int64
	PreviousLengthCm float64
	NewLengthCm      float64
	TrimmedCm        float64
}

// EventName returns the event type identifier.
func (e PetGroomed) EventName() string {
	return "pets.pet.groomed"
}

// PetDeleted is raised when a pet is removed from the store.
type PetDeleted struct {
	BaseEvent
	PetID int64
	Name  string
}

// EventName returns the event type identifier.
func (e PetDeleted) EventName() string {
	return "pets.pet.deleted"
}

// PetStatusChanged is raised when only the status changes.
type PetStatusChanged struct {
	BaseEvent
	PetID      int64
	FromStatus Status
	ToStatus   Status
}

// EventName returns the event type identifier.
func (e PetStatusChanged) EventName() string {
	return "pets.pet.status_changed"
}

// AggregateWithEvents is implemented by aggregates that track domain events.
type AggregateWithEvents interface {
	Events() []Event
	ClearEvents()
}
