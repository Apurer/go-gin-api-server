package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/ports"
)

type fakeStoreRepo struct {
	orders map[int64]*domain.Order
}

func newFakeStoreRepo() *fakeStoreRepo {
	return &fakeStoreRepo{orders: map[int64]*domain.Order{}}
}

func (f *fakeStoreRepo) Save(_ context.Context, order *domain.Order) (*domain.Order, error) {
	copy := *order
	f.orders[order.ID] = &copy
	return &copy, nil
}

func (f *fakeStoreRepo) GetByID(_ context.Context, id int64) (*domain.Order, error) {
	if o, ok := f.orders[id]; ok {
		copy := *o
		return &copy, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeStoreRepo) Delete(_ context.Context, id int64) error {
	if _, ok := f.orders[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.orders, id)
	return nil
}

func (f *fakeStoreRepo) List(_ context.Context) ([]*domain.Order, error) {
	var list []*domain.Order
	for _, o := range f.orders {
		copy := *o
		list = append(list, &copy)
	}
	return list, nil
}

func TestPlaceOrder_ValidatesAndPersists(t *testing.T) {
	repo := newFakeStoreRepo()
	svc := NewService(repo)

	order, err := domain.NewOrder(1, 10, 2, time.Now(), domain.StatusPlaced, false)
	require.NoError(t, err)

	saved, err := svc.PlaceOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, order.ID, saved.ID)
	require.Equal(t, domain.StatusPlaced, saved.Status)
}

func TestPlaceOrder_InvalidQuantity(t *testing.T) {
	repo := newFakeStoreRepo()
	svc := NewService(repo)

	order := &domain.Order{ID: 1, PetID: 1, Quantity: 0}
	_, err := svc.PlaceOrder(context.Background(), order)
	require.ErrorIs(t, err, domain.ErrInvalidQuantity)
}
