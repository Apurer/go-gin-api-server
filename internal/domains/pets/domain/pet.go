package domain

import (
	"errors"
	"strings"
)

// Status represents the lifecycle state of a pet inside the store catalog.
type Status string

const (
	StatusAvailable Status = "available"
	StatusPending   Status = "pending"
	StatusSold      Status = "sold"
)

// Category groups pets in the catalog.
type Category struct {
	ID   int64
	Name string
}

// Tag is a lightweight marker attached to pets for filtering.
type Tag struct {
	ID   int64
	Name string
}

// ExternalReference captures how the domain pet links to external providers.
type ExternalReference struct {
	Provider   string
	ID         string
	Attributes map[string]string
}

// GroomingOperation models the transient data required to compute the new hair length.
type GroomingOperation struct {
	InitialLengthCm float64
	TrimByCm        float64
}

// Pet represents the aggregate managed by the pets bounded context.
type Pet struct {
	ID           int64
	Category     *Category
	Name         string
	PhotoURLs    []string
	Tags         []Tag
	Status       Status
	HairLengthCm float64
	ExternalRef  *ExternalReference
}

var (
	ErrEmptyName       = errors.New("pet name is required")
	ErrEmptyPhotos     = errors.New("at least one photo url is required")
	ErrInvalidHair     = errors.New("hair length must be greater or equal to zero")
	ErrInvalidGrooming = errors.New("grooming operation must have a trim less than or equal to the initial length")
)

// NewPet validates the invariants and builds a new Pet aggregate.
func NewPet(id int64, name string, photoURLs []string) (*Pet, error) {
	p := &Pet{ID: id}
	if err := p.Rename(name); err != nil {
		return nil, err
	}
	if err := p.ReplacePhotos(photoURLs); err != nil {
		return nil, err
	}
	return p, nil
}

// Rename mutates the pet name ensuring the invariant.
func (p *Pet) Rename(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyName
	}
	p.Name = name
	return nil
}

// ReplacePhotos ensures at least one photo is stored.
func (p *Pet) ReplacePhotos(urls []string) error {
	if len(urls) == 0 {
		return ErrEmptyPhotos
	}
	p.PhotoURLs = append([]string{}, urls...)
	return nil
}

// UpdateHairLength stores the latest known hair length measurement.
func (p *Pet) UpdateHairLength(length float64) error {
	if length < 0 {
		return ErrInvalidHair
	}
	p.HairLengthCm = length
	return nil
}

// Groom applies a transient grooming operation, persisting only the result.
func (p *Pet) Groom(op GroomingOperation) error {
	if op.InitialLengthCm < 0 || op.TrimByCm < 0 {
		return ErrInvalidHair
	}
	if op.TrimByCm > op.InitialLengthCm {
		return ErrInvalidGrooming
	}
	return p.UpdateHairLength(op.InitialLengthCm - op.TrimByCm)
}

// UpdateStatus validates known lifecycle values.
func (p *Pet) UpdateStatus(status Status) {
	if status == "" {
		status = StatusAvailable
	}
	switch status {
	case StatusAvailable, StatusPending, StatusSold:
		p.Status = status
	default:
		p.Status = StatusAvailable
	}
}

// ReplaceTags swaps the current tag set.
func (p *Pet) ReplaceTags(tags []Tag) {
	p.Tags = append([]Tag{}, tags...)
}

// UpdateCategory sets a new category pointer.
func (p *Pet) UpdateCategory(cat *Category) {
	if cat == nil {
		p.Category = nil
		return
	}
	copy := *cat
	p.Category = &copy
}

// UpdateExternalReference stores the latest external provider linkage.
func (p *Pet) UpdateExternalReference(ref *ExternalReference) {
	if ref == nil {
		p.ExternalRef = nil
		return
	}
	copy := ExternalReference{Provider: ref.Provider, ID: ref.ID}
	if len(ref.Attributes) > 0 {
		copy.Attributes = make(map[string]string, len(ref.Attributes))
		for k, v := range ref.Attributes {
			copy.Attributes[k] = v
		}
	}
	p.ExternalRef = &copy
}

// Snapshot returns a defensive copy of the external reference.
func (p *Pet) SnapshotExternalReference() *ExternalReference {
	if p.ExternalRef == nil {
		return nil
	}
	copy := ExternalReference{Provider: p.ExternalRef.Provider, ID: p.ExternalRef.ID}
	if len(p.ExternalRef.Attributes) > 0 {
		copy.Attributes = make(map[string]string, len(p.ExternalRef.Attributes))
		for k, v := range p.ExternalRef.Attributes {
			copy.Attributes[k] = v
		}
	}
	return &copy
}
