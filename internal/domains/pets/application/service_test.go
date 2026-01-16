package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	petmemory "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/memory"
	pettypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	"github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
	"github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
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

func TestAddPet_PartnerSyncInvoked(t *testing.T) {
	repo := petmemory.NewRepository()
	syncer := &stubPartnerSync{}
	svc := NewService(repo, WithPartnerSync(syncer))

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        3,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, proj)
	require.True(t, syncer.called)
	require.Equal(t, int64(3), syncer.lastID)
}

func TestAddPet_PartnerSyncError(t *testing.T) {
	repo := petmemory.NewRepository()
	syncer := &stubPartnerSync{err: errors.New("boom")}
	svc := NewService(repo, WithPartnerSync(syncer))

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        4,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})

	require.ErrorIs(t, err, ErrPartnerSync)
	require.NotNil(t, proj, "pet should still be saved even if sync fails")
	require.True(t, syncer.called)
	require.Equal(t, int64(4), syncer.lastID)
}

func TestAddPet_IdempotencyReplaySkipsSync(t *testing.T) {
	repo := petmemory.NewRepository()
	store := petmemory.NewIdempotencyStore()
	syncer := &stubPartnerSync{}
	svc := NewService(repo, WithPartnerSync(syncer), WithIdempotencyStore(store))

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	input := pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        20,
			Name:      &name,
			PhotoURLs: &photos,
		},
		IdempotencyKey: "replay-key",
	}
	first, err := svc.AddPet(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, 1, syncer.callCount)

	syncer.called = false
	second, err := svc.AddPet(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.Pet.ID, second.Pet.ID)
	require.False(t, syncer.called, "replayed request should skip partner sync")
	require.Equal(t, 1, syncer.callCount)
}

func TestAddPet_IdempotencyConflict(t *testing.T) {
	repo := petmemory.NewRepository()
	store := petmemory.NewIdempotencyStore()
	svc := NewService(repo, WithIdempotencyStore(store))

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	key := "conflict-key"
	_, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        21,
			Name:      &name,
			PhotoURLs: &photos,
		},
		IdempotencyKey: key,
	})
	require.NoError(t, err)

	otherName := "Buddy"
	_, err = svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        22,
			Name:      &otherName,
			PhotoURLs: &photos,
		},
		IdempotencyKey: key,
	})
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

type stubPartnerSync struct {
	called    bool
	callCount int
	lastID    int64
	err       error
}

func (s *stubPartnerSync) Sync(_ context.Context, pet *domain.Pet) error {
	s.called = true
	s.callCount++
	if pet != nil {
		s.lastID = pet.ID
	}
	return s.err
}

func TestUpdatePetWithForm_PropagatesValidation(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Rex"
	photos := []string{"http://example.com/rex.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        10,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, proj)

	invalidName := "   "
	_, err = svc.UpdatePetWithForm(context.Background(), pettypes.UpdatePetWithFormInput{
		ID:   proj.Pet.ID,
		Name: &invalidName,
	})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestUpdatePetWithForm_UpdatesStatusAndName(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Luna"
	photos := []string{"http://example.com/luna.jpg"}
	created, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        11,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)

	updatedName := "Luna II"
	status := "pending"
	updated, err := svc.UpdatePetWithForm(context.Background(), pettypes.UpdatePetWithFormInput{
		ID:     created.Pet.ID,
		Name:   &updatedName,
		Status: &status,
	})
	require.NoError(t, err)
	require.Equal(t, updatedName, updated.Pet.Name)
	require.Equal(t, domain.StatusPending, updated.Pet.Status)
}

func TestGroomPet_InvalidOperation(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Milo"
	photos := []string{"http://example.com/milo.jpg"}
	proj, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        12,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)

	_, err = svc.GroomPet(context.Background(), pettypes.GroomPetInput{
		ID:                  proj.Pet.ID,
		InitialHairLengthCm: 2,
		TrimByCm:            3,
	})
	require.ErrorIs(t, err, ErrInvalidInput)
}

func TestFindByStatus_DefaultsToAvailable(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Buddy"
	photos := []string{"http://example.com/buddy.jpg"}
	_, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        13,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)

	otherName := "Shadow"
	_, err = svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        14,
			Name:      &otherName,
			PhotoURLs: &photos,
			Status: func() *string {
				val := string(domain.StatusPending)
				return &val
			}(),
		},
	})
	require.NoError(t, err)

	result, err := svc.FindByStatus(context.Background(), pettypes.FindPetsByStatusInput{})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(13), result[0].Pet.ID)
	require.Equal(t, domain.StatusAvailable, result[0].Pet.Status)
}

func TestFindByTags_CaseInsensitive(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Casey"
	photos := []string{"http://example.com/casey.jpg"}
	tags := []pettypes.TagInput{{Name: "Playful"}}
	_, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        15,
			Name:      &name,
			PhotoURLs: &photos,
			Tags:      &tags,
		},
	})
	require.NoError(t, err)

	result, err := svc.FindByTags(context.Background(), pettypes.FindPetsByTagsInput{Tags: []string{"playful"}})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(15), result[0].Pet.ID)
}

func TestDelete_Flows(t *testing.T) {
	repo := petmemory.NewRepository()
	svc := NewService(repo)

	name := "Temp"
	photos := []string{"http://example.com/temp.jpg"}
	created, err := svc.AddPet(context.Background(), pettypes.AddPetInput{
		PetMutationInput: pettypes.PetMutationInput{
			ID:        16,
			Name:      &name,
			PhotoURLs: &photos,
		},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), pettypes.PetIdentifier{ID: created.Pet.ID}))
	_, err = svc.GetByID(context.Background(), pettypes.PetIdentifier{ID: created.Pet.ID})
	require.ErrorIs(t, err, ports.ErrNotFound)

	err = svc.Delete(context.Background(), pettypes.PetIdentifier{ID: 999})
	require.ErrorIs(t, err, ports.ErrNotFound)
}
