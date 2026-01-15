package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	petmemory "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/memory"
	pettypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
)

func TestAddPet_Success(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        1,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, proj)
	require.Equal(t, int64(1), proj.Pet.ID)
	require.Equal(t, name, proj.Pet.Name)
	require.False(t, proj.Metadata.CreatedAt.IsZero())
	require.False(t, proj.Metadata.UpdatedAt.IsZero())
}

func TestAddPet_InvalidInput(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	_, err := svc.AddPet(context.Background(), pettypes.AddPetInput{})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestUpdatePet_UpdatesMetadata(t *testing.T) {
	repo := petmemory.NewRepository()
	repoNow := time.Now
	repo.WithClock(repoNow)
	svc := NewService(repo)

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        2,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)

	updatedName := "Rexy"
	updated, err := svc.UpdatePet(context.Background(), pettypes.UpdatePetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:   proj.Pet.ID,
			Name: &updatedName,
		},
	})
	require.NoError(t, err)
	require.Equal(t, updatedName, updated.Pet.Name)
	require.Equal(t, proj.Metadata.CreatedAt, updated.Metadata.CreatedAt)
	require.GreaterOrEqual(t, updated.Metadata.UpdatedAt, proj.Metadata.UpdatedAt)
}
