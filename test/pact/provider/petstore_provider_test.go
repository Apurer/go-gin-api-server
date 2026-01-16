//go:build pact
// +build pact

package provider_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	pacttest "github.com/Apurer/go-gin-api-server/test/pact"

	petstoreserver "github.com/Apurer/go-gin-api-server/generated/go"
	petsmemory "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/memory"
	petsobs "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/observability"
	petsworkflows "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/workflows"
	petsapp "github.com/Apurer/go-gin-api-server/internal/domains/pets/application"
	petdomain "github.com/Apurer/go-gin-api-server/internal/domains/pets/domain"
	storememory "github.com/Apurer/go-gin-api-server/internal/domains/store/adapters/memory"
	storeobs "github.com/Apurer/go-gin-api-server/internal/domains/store/adapters/observability"
	storeapp "github.com/Apurer/go-gin-api-server/internal/domains/store/application"
	usermemory "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/memory"
	userobs "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/observability"
	userapp "github.com/Apurer/go-gin-api-server/internal/domains/users/application"

	"github.com/gin-gonic/gin"
	"github.com/pact-foundation/pact-go/v2/models"
	pactprovider "github.com/pact-foundation/pact-go/v2/provider"
	"github.com/stretchr/testify/require"
)

func TestPetstoreProviderPact(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	app := newContractProviderApp(t)
	pactFile := filepath.ToSlash(pacttest.PactFile(t))
	if _, err := os.Stat(pactFile); errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pact file not found at %s - run the pact consumer tests first", pactFile)
	} else {
		require.NoError(t, err)
	}

	verifier := pactprovider.NewVerifier()
	stateHandlers := models.StateHandlers{
		pacttest.StatePetsBaseline: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetPets(t)
			return nil, nil
		},
		pacttest.StatePetExists: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetPets(t)
			if setup {
				app.seedPet(t, pacttest.ExistingPetID)
			}
			return nil, nil
		},
		pacttest.StatePetMissing: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetPets(t)
			return nil, nil
		},
	}

	err := verifier.VerifyProvider(t, pactprovider.VerifyRequest{
		ProviderBaseURL: app.server.URL,
		Provider:        pacttest.ProviderName,
		PactFiles:       []string{pactFile},
		StateHandlers:   stateHandlers,
		BeforeEach: func() error {
			app.resetPets(t)
			return nil
		},
	})
	require.NoError(t, err)
}

type contractProviderApp struct {
	repo   *petsmemory.Repository
	server *httptest.Server
}

func newContractProviderApp(t testing.TB) *contractProviderApp {
	t.Helper()

	petRepo := petsmemory.NewRepository()
	idempotencyStore := petsmemory.NewIdempotencyStore()
	petService := petsobs.New(petsapp.NewService(petRepo, petsapp.WithIdempotencyStore(idempotencyStore)))
	workflows := petsworkflows.NewInlinePetWorkflows(petService)

	storeService := storeobs.New(storeapp.NewService(storememory.NewRepository()))
	userService := userobs.New(userapp.NewService(usermemory.NewRepository(), usermemory.NewSessionStore()))

	handlers := petstoreserver.ApiHandleFunctions{
		PetAPI:   petstoreserver.NewPetAPI(petService, workflows, idempotencyStore),
		StoreAPI: petstoreserver.NewStoreAPI(storeService),
		UserAPI:  petstoreserver.NewUserAPI(userService),
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router = petstoreserver.NewRouterWithGinEngine(router, handlers)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	return &contractProviderApp{
		repo:   petRepo,
		server: server,
	}
}

func (a *contractProviderApp) resetPets(t testing.TB) {
	t.Helper()
	pets, err := a.repo.List(context.Background())
	require.NoError(t, err)
	for _, projection := range pets {
		_ = a.repo.Delete(context.Background(), projection.Pet.ID)
	}
}

func (a *contractProviderApp) seedPet(t testing.TB, id int64) {
	t.Helper()
	pet, err := petdomain.NewPet(id, "Fluffy Pact Cat", []string{"https://example.pact/pets/fluffy.png"})
	require.NoError(t, err)
	require.NoError(t, pet.UpdateStatus(petdomain.StatusAvailable))
	_, err = a.repo.Save(context.Background(), pet)
	require.NoError(t, err)
}
