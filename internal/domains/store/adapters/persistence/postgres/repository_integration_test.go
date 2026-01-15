//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/store/ports"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/migrations"
)

func setupStorePostgresContainer(t *testing.T) (*gorm.DB, func()) {
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

func TestRepository_SaveAndGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupStorePostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	order, err := domain.NewOrder(1, 10, 2, time.Now(), domain.StatusPlaced, false)
	require.NoError(t, err)

	saved, err := repo.Save(ctx, order)
	require.NoError(t, err)
	assert.Equal(t, order.ID, saved.ID)
	assert.Equal(t, order.Status, saved.Status)

	fetched, err := repo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, order.ID, fetched.ID)
	assert.Equal(t, order.PetID, fetched.PetID)
}

func TestRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupStorePostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	order, err := domain.NewOrder(1, 10, 2, time.Now(), domain.StatusPlaced, false)
	require.NoError(t, err)
	_, err = repo.Save(ctx, order)
	require.NoError(t, err)

	err = order.UpdateStatus(domain.StatusApproved)
	require.NoError(t, err)
	updated, err := repo.Save(ctx, order)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusApproved, updated.Status)
}

func TestRepository_List(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupStorePostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		order, err := domain.NewOrder(i, i*10, 1, time.Now(), domain.StatusPlaced, false)
		require.NoError(t, err)
		_, err = repo.Save(ctx, order)
		require.NoError(t, err)
	}

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupStorePostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	order, err := domain.NewOrder(1, 10, 2, time.Now(), domain.StatusPlaced, false)
	require.NoError(t, err)
	_, err = repo.Save(ctx, order)
	require.NoError(t, err)

	err = repo.Delete(ctx, order.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, order.ID)
	assert.ErrorIs(t, err, ports.ErrNotFound)

	err = repo.Delete(ctx, order.ID)
	assert.ErrorIs(t, err, ports.ErrNotFound)
}
