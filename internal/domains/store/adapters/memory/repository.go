package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/ports"
)

var _ ports.Repository = (*Repository)(nil)

// Repository is an in-memory order persistence adapter.
type Repository struct {
	mu     sync.RWMutex
	orders map[int64]*domain.Order
	nextID int64
}

func NewRepository() *Repository {
	return &Repository{orders: map[int64]*domain.Order{}}
}

func (r *Repository) Save(_ context.Context, order *domain.Order) (*domain.Order, error) {
	if order == nil {
		return nil, errors.New("order is nil")
	}
	clone := *order
	if err := clone.UpdateStatus(clone.Status); err != nil {
		return nil, err
	}
	if err := clone.Validate(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if clone.ID == 0 {
		r.nextID++
		clone.ID = r.nextID
	} else if clone.ID > r.nextID {
		r.nextID = clone.ID
	}
	r.orders[clone.ID] = &clone
	return &clone, nil
}

func (r *Repository) GetByID(_ context.Context, id int64) (*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	order, ok := r.orders[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	clone := *order
	return &clone, nil
}

func (r *Repository) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.orders[id]; !ok {
		return ports.ErrNotFound
	}
	delete(r.orders, id)
	return nil
}

func (r *Repository) List(_ context.Context) ([]*domain.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*domain.Order, 0, len(r.orders))
	for _, order := range r.orders {
		clone := *order
		list = append(list, &clone)
	}
	return list, nil
}
