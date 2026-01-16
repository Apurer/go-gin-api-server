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
	"time"

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
	storedomain "github.com/Apurer/go-gin-api-server/internal/domains/store/domain"
	usermemory "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/memory"
	userobs "github.com/Apurer/go-gin-api-server/internal/domains/users/adapters/observability"
	userapp "github.com/Apurer/go-gin-api-server/internal/domains/users/application"
	userdomain "github.com/Apurer/go-gin-api-server/internal/domains/users/domain"

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
		pacttest.StatePetsSearch: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetPets(t)
			if setup {
				app.seedSearchPets(t)
			}
			return nil, nil
		},
		pacttest.StateOrdersBase: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetStore(t)
			return nil, nil
		},
		pacttest.StateOrderExists: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetStore(t)
			if setup {
				app.seedOrder(t, pacttest.ExistingOrderID, pacttest.ExistingPetID)
			}
			return nil, nil
		},
		pacttest.StateInventory: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetStore(t)
			if setup {
				app.seedInventory(t)
			}
			return nil, nil
		},
		pacttest.StateUsersBase: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetUsers(t)
			return nil, nil
		},
		pacttest.StateUserExists: func(setup bool, _ models.ProviderState) (models.ProviderStateResponse, error) {
			app.resetUsers(t)
			if setup {
				app.seedUser(t, pacttest.UserPrimaryUsername, pacttest.UserPassword)
			}
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
			app.resetStore(t)
			app.resetUsers(t)
			return nil
		},
	})
	require.NoError(t, err)
}

type contractProviderApp struct {
	petRepo     *petsmemory.Repository
	storeRepo   *storememory.Repository
	userRepo    *usermemory.Repository
	sessionRepo *usermemory.SessionStore
	server      *httptest.Server
}

func newContractProviderApp(t testing.TB) *contractProviderApp {
	t.Helper()

	petRepo := petsmemory.NewRepository()
	idempotencyStore := petsmemory.NewIdempotencyStore()
	petService := petsobs.New(petsapp.NewService(petRepo, petsapp.WithIdempotencyStore(idempotencyStore)))
	workflows := petsworkflows.NewInlinePetWorkflows(petService)

	storeRepo := storememory.NewRepository()
	storeService := storeobs.New(storeapp.NewService(storeRepo))

	userRepo := usermemory.NewRepository()
	sessionStore := usermemory.NewSessionStore()
	userService := userobs.New(userapp.NewService(userRepo, sessionStore))

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
		petRepo:     petRepo,
		storeRepo:   storeRepo,
		userRepo:    userRepo,
		sessionRepo: sessionStore,
		server:      server,
	}
}

func (a *contractProviderApp) resetPets(t testing.TB) {
	t.Helper()
	pets, err := a.petRepo.List(context.Background())
	require.NoError(t, err)
	for _, projection := range pets {
		_ = a.petRepo.Delete(context.Background(), projection.Pet.ID)
	}
}

func (a *contractProviderApp) seedPet(t testing.TB, id int64) {
	t.Helper()
	pet, err := petdomain.NewPet(id, "Fluffy Pact Cat", []string{"https://example.pact/pets/fluffy.png"})
	require.NoError(t, err)
	require.NoError(t, pet.UpdateStatus(petdomain.StatusAvailable))
	_, err = a.petRepo.Save(context.Background(), pet)
	require.NoError(t, err)
}

func (a *contractProviderApp) seedSearchPets(t testing.TB) {
	t.Helper()
	create := func(id int64, name string, status petdomain.Status, tags []string) {
		pet, err := petdomain.NewPet(id, name, []string{"https://example.pact/pets/searchable.png"})
		require.NoError(t, err)
		require.NoError(t, pet.UpdateStatus(status))
		var domainTags []petdomain.Tag
		for i, tag := range tags {
			domainTags = append(domainTags, petdomain.Tag{ID: int64(i + 1), Name: tag})
		}
		pet.ReplaceTags(domainTags)
		_, err = a.petRepo.Save(context.Background(), pet)
		require.NoError(t, err)
	}
	create(pacttest.SearchPetID, "Searchable Pact Pet", petdomain.StatusAvailable, []string{"pact", "featured"})
	create(pacttest.ExistingPetID+50, "Pending Pact Pet", petdomain.StatusPending, []string{"pact"})
}

func (a *contractProviderApp) resetStore(t testing.TB) {
	t.Helper()
	orders, err := a.storeRepo.List(context.Background())
	require.NoError(t, err)
	for _, order := range orders {
		_ = a.storeRepo.Delete(context.Background(), order.ID)
	}
}

func (a *contractProviderApp) seedOrder(t testing.TB, id int64, petID int64) {
	t.Helper()
	order, err := storedomain.NewOrder(id, petID, 2, time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC), storedomain.StatusApproved, true)
	require.NoError(t, err)
	_, err = a.storeRepo.Save(context.Background(), order)
	require.NoError(t, err)
}

func (a *contractProviderApp) seedInventory(t testing.TB) {
	t.Helper()
	a.seedOrder(t, 401, pacttest.ExistingPetID)
	orderPlaced, err := storedomain.NewOrder(402, pacttest.SearchPetID, 3, time.Now().UTC(), storedomain.StatusPlaced, false)
	require.NoError(t, err)
	_, err = a.storeRepo.Save(context.Background(), orderPlaced)
	require.NoError(t, err)
	delivered, err := storedomain.NewOrder(403, pacttest.ExistingPetID, 2, time.Now().UTC(), storedomain.StatusDelivered, true)
	require.NoError(t, err)
	_, err = a.storeRepo.Save(context.Background(), delivered)
	require.NoError(t, err)
}

func (a *contractProviderApp) resetUsers(t testing.TB) {
	t.Helper()
	users, err := a.userRepo.List(context.Background())
	require.NoError(t, err)
	for _, user := range users {
		_ = a.userRepo.Delete(context.Background(), user.Username)
	}
}

func (a *contractProviderApp) seedUser(t testing.TB, username, password string) {
	t.Helper()
	user, err := userdomain.NewUser(501, username, password)
	require.NoError(t, err)
	require.NoError(t, user.UpdateProfile("Pact", "User", "pact.user@example.com", "+1234567890"))
	user.UpdateStatus(1)
	_, err = a.userRepo.Save(context.Background(), user)
	require.NoError(t, err)
}
