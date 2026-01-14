package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/ports"
)

var _ ports.Repository = (*Repository)(nil)

// Repository persists orders in PostgreSQL using GORM.
type Repository struct {
	db *gorm.DB
}

// NewRepository wires a PostgreSQL-backed repository. Caller manages DB lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	repo := &Repository{db: db}
	if db != nil {
		_ = db.AutoMigrate(&orderRecord{})
	}
	return repo
}

// orderRecord maps the order aggregate to a relational table.
type orderRecord struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	PetID     int64     `gorm:"column:pet_id;index:idx_orders_status_pet"`
	Quantity  int32     `gorm:"column:quantity"`
	ShipDate  time.Time `gorm:"column:ship_date"`
	Status    string    `gorm:"column:status;type:varchar(32);index:idx_orders_status_pet"`
	Complete  bool      `gorm:"column:complete"`
	CreatedAt time.Time `gorm:"column:created_at;index"`
	UpdatedAt time.Time `gorm:"column:updated_at;index"`
}

func (orderRecord) TableName() string { return "orders" }

// Save inserts or updates an order.
func (r *Repository) Save(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("order is nil")
	}
	record := toRecord(order)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"pet_id":     record.PetID,
				"quantity":   record.Quantity,
				"ship_date":  record.ShipDate,
				"status":     record.Status,
				"complete":   record.Complete,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).Create(&record).Error; err != nil {
		return nil, err
	}
	return r.GetByID(ctx, record.ID)
}

// GetByID fetches an order by identifier.
func (r *Repository) GetByID(ctx context.Context, id int64) (*domain.Order, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record orderRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return record.toDomain(), nil
}

// Delete removes an order by identifier.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Delete(&orderRecord{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// List returns all orders.
func (r *Repository) List(ctx context.Context) ([]*domain.Order, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []orderRecord
	if err := r.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	orders := make([]*domain.Order, 0, len(records))
	for i := range records {
		orders = append(orders, records[i].toDomain())
	}
	return orders, nil
}

func (r *Repository) ensureDB() error {
	if r == nil || r.db == nil {
		return errors.New("postgres order repository not configured")
	}
	return nil
}

func toRecord(order *domain.Order) orderRecord {
	rec := orderRecord{
		ID:       order.ID,
		PetID:    order.PetID,
		Quantity: order.Quantity,
		ShipDate: order.ShipDate,
		Status:   string(order.Status),
		Complete: order.Complete,
	}
	return rec
}

func (r orderRecord) toDomain() *domain.Order {
	return &domain.Order{
		ID:       r.ID,
		PetID:    r.PetID,
		Quantity: r.Quantity,
		ShipDate: r.ShipDate,
		Status:   domain.Status(r.Status),
		Complete: r.Complete,
	}
}
