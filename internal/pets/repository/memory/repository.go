package memory

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/ports"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/shared/projection"
)

var _ ports.Repository = (*Repository)(nil)

// Repository is an in-memory implementation used for demos/tests.
type Repository struct {
	mu   sync.RWMutex
	pets map[int64]*storedPet
	now  func() time.Time
}

type storedPet struct {
	pet      *domain.Pet
	metadata projection.Metadata
}

// NewRepository constructs an empty in-memory store.
func NewRepository() *Repository {
	return &Repository{
		pets: map[int64]*storedPet{},
		now:  time.Now,
	}
}

// Save inserts or replaces a pet while maintaining metadata.
func (r *Repository) Save(_ context.Context, pet *domain.Pet) (*projection.Projection[*domain.Pet], error) {
	if pet == nil {
		return nil, errors.New("cannot save nil pet")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.pets[pet.ID]
	timestamp := r.now()
	metadata := projection.Metadata{CreatedAt: timestamp, UpdatedAt: timestamp}
	if ok {
		metadata.CreatedAt = entry.metadata.CreatedAt
	}

	stored := &storedPet{
		pet:      clonePet(pet),
		metadata: metadata,
	}
	r.pets[pet.ID] = stored
	return projectionCopy(stored), nil
}

// GetByID fetches a pet if present.
func (r *Repository) GetByID(_ context.Context, id int64) (*projection.Projection[*domain.Pet], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.pets[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return projectionCopy(entry), nil
}

// Delete removes a pet.
func (r *Repository) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pets[id]; !ok {
		return ports.ErrNotFound
	}
	delete(r.pets, id)
	return nil
}

// FindByStatus returns pets with matching status.
func (r *Repository) FindByStatus(_ context.Context, statuses []domain.Status) ([]*projection.Projection[*domain.Pet], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := map[domain.Status]struct{}{}
	for _, s := range statuses {
		set[s] = struct{}{}
	}
	var list []*projection.Projection[*domain.Pet]
	for _, entry := range r.pets {
		if _, ok := set[entry.pet.Status]; ok {
			list = append(list, projectionCopy(entry))
		}
	}
	return list, nil
}

// FindByTags returns pets with overlapping tags.
func (r *Repository) FindByTags(_ context.Context, tags []string) ([]*projection.Projection[*domain.Pet], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(tags) == 0 {
		return nil, nil
	}
	lookup := map[string]struct{}{}
	for _, t := range tags {
		lookup[strings.ToLower(t)] = struct{}{}
	}
	var list []*projection.Projection[*domain.Pet]
	for _, entry := range r.pets {
		for _, tag := range entry.pet.Tags {
			if _, ok := lookup[strings.ToLower(tag.Name)]; ok {
				list = append(list, projectionCopy(entry))
				break
			}
		}
	}
	return list, nil
}

// List returns all pets.
func (r *Repository) List(_ context.Context) ([]*projection.Projection[*domain.Pet], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*projection.Projection[*domain.Pet], 0, len(r.pets))
	for _, entry := range r.pets {
		list = append(list, projectionCopy(entry))
	}
	return list, nil
}

func projectionCopy(entry *storedPet) *projection.Projection[*domain.Pet] {
	return &projection.Projection[*domain.Pet]{
		Entity:   clonePet(entry.pet),
		Metadata: entry.metadata,
	}
}

func clonePet(p *domain.Pet) *domain.Pet {
	if p == nil {
		return nil
	}
	clone := *p
	if p.Category != nil {
		category := *p.Category
		clone.Category = &category
	}
	if len(p.PhotoURLs) > 0 {
		clone.PhotoURLs = append([]string{}, p.PhotoURLs...)
	}
	if len(p.Tags) > 0 {
		clone.Tags = append([]domain.Tag{}, p.Tags...)
	}
	if p.ExternalRef != nil {
		ref := domain.ExternalReference{Provider: p.ExternalRef.Provider, ID: p.ExternalRef.ID}
		if len(p.ExternalRef.Attributes) > 0 {
			ref.Attributes = make(map[string]string, len(p.ExternalRef.Attributes))
			for k, v := range p.ExternalRef.Attributes {
				ref.Attributes[k] = v
			}
		}
		clone.ExternalRef = &ref
	}
	return &clone
}
