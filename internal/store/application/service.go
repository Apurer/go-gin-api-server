package application

import (
	"context"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/store/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/store/ports"
)

// Service orchestrates store/order use cases.
type Service struct {
	repo ports.Repository
}

func NewService(repo ports.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) PlaceOrder(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	return s.repo.Save(ctx, order)
}

func (s *Service) GetOrderByID(ctx context.Context, id int64) (*domain.Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) DeleteOrder(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// Inventory returns the quantity of orders by status (used as store inventory proxy).
func (s *Service) Inventory(ctx context.Context) (map[string]int32, error) {
	orders, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := map[string]int32{}
	for _, order := range orders {
		result[string(order.Status)] += order.Quantity
	}
	return result, nil
}
