package postgres

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/pets/ports"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/shared/projection"
)

var _ ports.Repository = (*Repository)(nil)

// Repository persists pets in PostgreSQL using GORM-mapped columns.
type Repository struct {
	db *gorm.DB
}

// NewRepository wires a PostgreSQL-backed repository. The caller owns the DB lifecycle.
func NewRepository(db *gorm.DB) *Repository {
	repo := &Repository{db: db}
	if db != nil {
		if err := db.AutoMigrate(&petRecord{}); err != nil {
			log.Printf("postgres repository migration failed: %v", err)
		}
	}
	return repo
}

type petRecord struct {
	ID                 int64             `gorm:"primaryKey;column:id"`
	CategoryID         *int64            `gorm:"column:category_id"`
	CategoryName       string            `gorm:"column:category_name"`
	Name               string            `gorm:"column:name"`
	PhotoURLs          pq.StringArray    `gorm:"column:photo_urls;type:text[]"`
	Status             string            `gorm:"column:status;type:varchar(32);index"`
	HairLengthCm       float64           `gorm:"column:hair_length_cm"`
	TagIDs             pq.Int64Array     `gorm:"column:tag_ids;type:bigint[]"`
	TagNames           pq.StringArray    `gorm:"column:tag_names;type:text[]"`
	ExternalProvider   string            `gorm:"column:external_provider"`
	ExternalID         string            `gorm:"column:external_id"`
	ExternalAttributes map[string]string `gorm:"column:external_attributes;serializer:json"`
	CreatedAt          time.Time         `gorm:"column:created_at"`
	UpdatedAt          time.Time         `gorm:"column:updated_at"`
}

func (petRecord) TableName() string { return "pets" }

func newPetRecord(p *domain.Pet) petRecord {
	rec := petRecord{
		ID:           p.ID,
		Name:         p.Name,
		Status:       string(p.Status),
		HairLengthCm: p.HairLengthCm,
		PhotoURLs:    copyStringArray(p.PhotoURLs),
		TagIDs:       extractTagIDs(p.Tags),
		TagNames:     extractTagNames(p.Tags),
	}
	if p.Category != nil {
		rec.CategoryID = cloneInt64Ptr(p.Category.ID)
		rec.CategoryName = p.Category.Name
	}
	if p.ExternalRef != nil {
		rec.ExternalProvider = p.ExternalRef.Provider
		rec.ExternalID = p.ExternalRef.ID
		if len(p.ExternalRef.Attributes) > 0 {
			rec.ExternalAttributes = make(map[string]string, len(p.ExternalRef.Attributes))
			for k, v := range p.ExternalRef.Attributes {
				rec.ExternalAttributes[k] = v
			}
		}
	}
	return rec
}

// Save inserts or updates a pet aggregate.
func (r *Repository) Save(ctx context.Context, pet *domain.Pet) (*projection.Projection[*domain.Pet], error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if pet == nil {
		return nil, errors.New("cannot save nil pet")
	}
	record := newPetRecord(pet)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"category_id":         record.CategoryID,
				"category_name":       record.CategoryName,
				"name":                record.Name,
				"photo_urls":          record.PhotoURLs,
				"status":              record.Status,
				"hair_length_cm":      record.HairLengthCm,
				"tag_ids":             record.TagIDs,
				"tag_names":           record.TagNames,
				"external_provider":   record.ExternalProvider,
				"external_id":         record.ExternalID,
				"external_attributes": record.ExternalAttributes,
				"updated_at":          gorm.Expr("NOW()"),
			}),
		}).Create(&record).Error; err != nil {
		return nil, err
	}
	return r.GetByID(ctx, pet.ID)
}

// GetByID fetches a pet by identifier.
func (r *Repository) GetByID(ctx context.Context, id int64) (*projection.Projection[*domain.Pet], error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record petRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return toProjection(&record)
}

// Delete removes a pet by identifier.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Delete(&petRecord{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// FindByStatus returns pets matching any provided status.
func (r *Repository) FindByStatus(ctx context.Context, statuses []domain.Status) ([]*projection.Projection[*domain.Pet], error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	args := make([]string, 0, len(statuses))
	for _, s := range statuses {
		args = append(args, string(s))
	}
	var records []petRecord
	if err := r.db.WithContext(ctx).
		Where("status IN ?", args).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return recordsToProjections(records)
}

// FindByTags returns pets that contain any of the provided tag names (case insensitive).
func (r *Repository) FindByTags(ctx context.Context, tags []string) ([]*projection.Projection[*domain.Pet], error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}
	lowered := make([]string, 0, len(tags))
	for _, tag := range tags {
		lowered = append(lowered, strings.ToLower(tag))
	}
	var records []petRecord
	if err := r.db.WithContext(ctx).
		Where("EXISTS (SELECT 1 FROM unnest(tag_names) AS tag WHERE lower(tag) = ANY(?))", pq.Array(lowered)).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return recordsToProjections(records)
}

// List returns every persisted pet.
func (r *Repository) List(ctx context.Context) ([]*projection.Projection[*domain.Pet], error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var records []petRecord
	if err := r.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	return recordsToProjections(records)
}

func recordsToProjections(records []petRecord) ([]*projection.Projection[*domain.Pet], error) {
	list := make([]*projection.Projection[*domain.Pet], 0, len(records))
	for i := range records {
		projection, err := toProjection(&records[i])
		if err != nil {
			return nil, err
		}
		list = append(list, projection)
	}
	return list, nil
}

func toProjection(record *petRecord) (*projection.Projection[*domain.Pet], error) {
	if record == nil {
		return nil, nil
	}
	pet := record.toDomain()
	return &projection.Projection[*domain.Pet]{
		Entity:   pet,
		Metadata: projection.Metadata{CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt},
	}, nil
}

func (r *petRecord) toDomain() *domain.Pet {
	if r == nil {
		return nil
	}
	pet := &domain.Pet{
		ID:           r.ID,
		Name:         r.Name,
		Status:       domain.Status(r.Status),
		HairLengthCm: r.HairLengthCm,
	}
	if len(r.PhotoURLs) > 0 {
		pet.PhotoURLs = append([]string{}, r.PhotoURLs...)
	}
	if r.CategoryID != nil || r.CategoryName != "" {
		cat := domain.Category{Name: r.CategoryName}
		if r.CategoryID != nil {
			cat.ID = *r.CategoryID
		}
		pet.Category = &cat
	}
	if n := max(len(r.TagIDs), len(r.TagNames)); n > 0 {
		tags := make([]domain.Tag, 0, n)
		for i := 0; i < n; i++ {
			var tag domain.Tag
			if i < len(r.TagIDs) {
				tag.ID = r.TagIDs[i]
			}
			if i < len(r.TagNames) {
				tag.Name = r.TagNames[i]
			}
			tags = append(tags, tag)
		}
		pet.Tags = tags
	}
	if r.ExternalProvider != "" || r.ExternalID != "" || len(r.ExternalAttributes) > 0 {
		reference := domain.ExternalReference{
			Provider: r.ExternalProvider,
			ID:       r.ExternalID,
		}
		if len(r.ExternalAttributes) > 0 {
			reference.Attributes = make(map[string]string, len(r.ExternalAttributes))
			for k, v := range r.ExternalAttributes {
				reference.Attributes[k] = v
			}
		}
		pet.ExternalRef = &reference
	}
	return clonePet(pet)
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

func (r *Repository) ensureDB() error {
	if r == nil || r.db == nil {
		return errors.New("postgres repository not configured")
	}
	return nil
}

func extractTagNames(tags []domain.Tag) pq.StringArray {
	if len(tags) == 0 {
		return nil
	}
	arr := make(pq.StringArray, 0, len(tags))
	for _, tag := range tags {
		if tag.Name == "" {
			continue
		}
		arr = append(arr, tag.Name)
	}
	return arr
}

func extractTagIDs(tags []domain.Tag) pq.Int64Array {
	if len(tags) == 0 {
		return nil
	}
	arr := make(pq.Int64Array, 0, len(tags))
	for _, tag := range tags {
		arr = append(arr, tag.ID)
	}
	return arr
}

func copyStringArray(values []string) pq.StringArray {
	if len(values) == 0 {
		return nil
	}
	dup := append([]string{}, values...)
	return pq.StringArray(dup)
}

func cloneInt64Ptr(v int64) *int64 {
	value := v
	return &value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
