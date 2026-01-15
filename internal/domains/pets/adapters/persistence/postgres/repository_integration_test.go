//go:build integration
// +build integration

// To enable gopls support for this file, add the following to your VSCode settings.json:
// "gopls": {
//   "buildFlags": ["-tags=integration"]
// }

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	petspostgres "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/persistence/postgres"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/migrations"
)

func setupPostgresContainer(t *testing.T) (*gorm.DB, func()) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		tcpostgres.WithDatabase("petstore_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = migrations.Run(db)
	require.NoError(t, err)

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		pgContainer.Terminate(ctx)
	}

	return db, cleanup
}

func TestPostgresRepository_SaveAndGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create a pet
	pet, err := domain.NewPet(1, "Buddy", []string{"http://example.com/buddy.jpg"})
	require.NoError(t, err)
	pet.UpdateStatus(domain.StatusAvailable)
	pet.UpdateCategory(&domain.Category{ID: 1, Name: "Dogs"})
	pet.ReplaceTags([]domain.Tag{{ID: 1, Name: "friendly"}, {ID: 2, Name: "trained"}})

	// Save
	projection, err := repo.Save(ctx, pet)
	require.NoError(t, err)
	assert.NotNil(t, projection)
	assert.Equal(t, "Buddy", projection.Pet.Name)
	assert.False(t, projection.Metadata.CreatedAt.IsZero())
	assert.False(t, projection.Metadata.UpdatedAt.IsZero())

	// Get by ID
	retrieved, err := repo.GetByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Buddy", retrieved.Pet.Name)
	assert.Equal(t, domain.StatusAvailable, retrieved.Pet.Status)
	assert.Equal(t, "Dogs", retrieved.Pet.Category.Name)
	assert.Len(t, retrieved.Pet.Tags, 2)
}

func TestPostgresRepository_FindByStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create pets with different statuses
	pets := []struct {
		id     int64
		name   string
		status domain.Status
	}{
		{1, "Available Dog", domain.StatusAvailable},
		{2, "Pending Cat", domain.StatusPending},
		{3, "Sold Bird", domain.StatusSold},
		{4, "Another Available", domain.StatusAvailable},
	}

	for _, p := range pets {
		pet, err := domain.NewPet(p.id, p.name, []string{"http://example.com/photo.jpg"})
		require.NoError(t, err)
		pet.UpdateStatus(p.status)
		_, err = repo.Save(ctx, pet)
		require.NoError(t, err)
	}

	// Find available pets
	available, err := repo.FindByStatus(ctx, []domain.Status{domain.StatusAvailable})
	require.NoError(t, err)
	assert.Len(t, available, 2)

	// Find pending and sold
	pendingAndSold, err := repo.FindByStatus(ctx, []domain.Status{domain.StatusPending, domain.StatusSold})
	require.NoError(t, err)
	assert.Len(t, pendingAndSold, 2)
}

func TestPostgresRepository_FindByTags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create pets with tags
	pet1, err := domain.NewPet(1, "Friendly Dog", []string{"http://example.com/dog.jpg"})
	require.NoError(t, err)
	pet1.ReplaceTags([]domain.Tag{{ID: 1, Name: "friendly"}, {ID: 2, Name: "trained"}})
	_, err = repo.Save(ctx, pet1)
	require.NoError(t, err)

	pet2, err := domain.NewPet(2, "Lazy Cat", []string{"http://example.com/cat.jpg"})
	require.NoError(t, err)
	pet2.ReplaceTags([]domain.Tag{{ID: 3, Name: "lazy"}, {ID: 4, Name: "indoor"}})
	_, err = repo.Save(ctx, pet2)
	require.NoError(t, err)

	// Find by tag
	result, err := repo.FindByTags(ctx, []string{"friendly"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Friendly Dog", result[0].Pet.Name)
}

func TestPostgresRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create and save a pet
	pet, err := domain.NewPet(1, "ToDelete", []string{"http://example.com/photo.jpg"})
	require.NoError(t, err)
	_, err = repo.Save(ctx, pet)
	require.NoError(t, err)

	// Delete
	err = repo.Delete(ctx, 1)
	require.NoError(t, err)

	// Verify not found
	_, err = repo.GetByID(ctx, 1)
	assert.ErrorIs(t, err, ports.ErrNotFound)

	// Delete again should error
	err = repo.Delete(ctx, 1)
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestPostgresRepository_List(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create multiple pets
	for i := int64(1); i <= 5; i++ {
		pet, err := domain.NewPet(i, fmt.Sprintf("Pet %d", i), []string{"http://example.com/photo.jpg"})
		require.NoError(t, err)
		_, err = repo.Save(ctx, pet)
		require.NoError(t, err)
	}

	// List all
	all, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 5)
}

func TestPostgresRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := petspostgres.NewRepository(db)
	ctx := context.Background()

	// Create and save
	pet, err := domain.NewPet(1, "Original Name", []string{"http://example.com/photo.jpg"})
	require.NoError(t, err)
	pet.UpdateStatus(domain.StatusAvailable)
	saved, err := repo.Save(ctx, pet)
	require.NoError(t, err)
	originalCreatedAt := saved.Metadata.CreatedAt

	// Sleep briefly to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Update
	err = pet.Rename("Updated Name")
	require.NoError(t, err)
	pet.UpdateStatus(domain.StatusPending)
	updated, err := repo.Save(ctx, pet)
	require.NoError(t, err)

	assert.Equal(t, "Updated Name", updated.Pet.Name)
	assert.Equal(t, domain.StatusPending, updated.Pet.Status)
	assert.Equal(t, originalCreatedAt.Unix(), updated.Metadata.CreatedAt.Unix())
	assert.True(t, updated.Metadata.UpdatedAt.After(originalCreatedAt))
}
