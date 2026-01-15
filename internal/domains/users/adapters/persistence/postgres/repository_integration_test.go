//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/ports"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/platform/migrations"
)

func setupUsersPostgresContainer(t *testing.T) (*gorm.DB, func()) {
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

func TestRepository_SaveAndGetByUsername(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupUsersPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	user, err := domain.NewUser(1, "alice", "secret")
	require.NoError(t, err)
	err = user.UpdateProfile("Alice", "Doe", "alice@example.com", "1234")
	require.NoError(t, err)

	saved, err := repo.Save(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, "alice", saved.Username)
	assert.Equal(t, "Alice", saved.FirstName)

	fetched, err := repo.GetByUsername(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, saved.ID, fetched.ID)
	assert.Equal(t, saved.Email, fetched.Email)
}

func TestRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupUsersPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	user, err := domain.NewUser(1, "alice", "secret")
	require.NoError(t, err)
	_, err = repo.Save(ctx, user)
	require.NoError(t, err)

	user.UpdateStatus(2)
	err = user.UpdateProfile("Alice", "Smith", "alice.smith@example.com", "9876")
	require.NoError(t, err)

	updated, err := repo.Save(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, int32(2), updated.Status)
	assert.Equal(t, "alice.smith@example.com", updated.Email)
}

func TestRepository_ListAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db, cleanup := setupUsersPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(db)
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		username := fmt.Sprintf("user%d", i)
		user, err := domain.NewUser(i, username, "pw123")
		require.NoError(t, err)
		_, err = repo.Save(ctx, user)
		require.NoError(t, err)
	}

	users, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 3)

	err = repo.Delete(ctx, "user2")
	require.NoError(t, err)
	_, err = repo.GetByUsername(ctx, "user2")
	assert.ErrorIs(t, err, ports.ErrNotFound)

	err = repo.Delete(ctx, "user2")
	assert.ErrorIs(t, err, ports.ErrNotFound)
}
