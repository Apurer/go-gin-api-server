//go:build pact
// +build pact

package consumer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	pacttest "github.com/Apurer/go-gin-api-server/test/pact"

	pactconsumer "github.com/pact-foundation/pact-go/v2/consumer"
	pactlog "github.com/pact-foundation/pact-go/v2/log"
	"github.com/pact-foundation/pact-go/v2/matchers"
	"github.com/stretchr/testify/require"
)

type petPayload struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	PhotoURLs    []string `json:"photoUrls"`
	Status       string   `json:"status"`
	HairLengthCm float64  `json:"hairLengthCm,omitempty"`
}

type problemDetail struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

type apiError struct {
	status int
	title  string
	detail string
}

func (e apiError) Error() string {
	msg := e.title
	if msg == "" {
		msg = "api error"
	}
	if e.detail != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.detail)
	}
	return fmt.Sprintf("%s (status %d)", msg, e.status)
}

func (e apiError) Status() int {
	return e.status
}

type orderPayload struct {
	ID       int64     `json:"id"`
	PetID    int64     `json:"petId"`
	Quantity int32     `json:"quantity"`
	ShipDate time.Time `json:"shipDate"`
	Status   string    `json:"status"`
	Complete bool      `json:"complete"`
}

type userPayload struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Phone      string `json:"phone"`
	UserStatus int32  `json:"userStatus"`
}

type groomingPayload struct {
	InitialHairLengthCm float64 `json:"initialHairLengthCm"`
	TrimByCm            float64 `json:"trimByCm"`
}

func TestPetPortalContract(t *testing.T) {
	t.Helper()
	pactlog.SetLogLevel("INFO")

	pact, err := pactconsumer.NewV2Pact(pactconsumer.MockHTTPProviderConfig{
		Consumer: pacttest.ConsumerName,
		Provider: pacttest.ProviderName,
		PactDir:  pacttest.PactDir(t),
		LogDir:   pacttest.LogDir(t),
	})
	require.NoError(t, err)

	requestPet := petPayload{
		ID:        pacttest.ExistingPetID,
		Name:      "Fluffy Pact Cat",
		PhotoURLs: []string{"https://example.pact/pets/fluffy.png"},
		Status:    "available",
	}
	updatePet := petPayload{
		ID:           pacttest.ExistingPetID,
		Name:         "Fluffy Pact Cat v2",
		PhotoURLs:    []string{"https://example.pact/pets/fluffy.png", "https://example.pact/pets/fluffy-2.png"},
		Status:       "sold",
		HairLengthCm: 1.5,
	}
	petBodyMatcher := matchers.Map{
		"id":        matchers.Like(requestPet.ID),
		"name":      matchers.Like(requestPet.Name),
		"photoUrls": matchers.ArrayMinLike(requestPet.PhotoURLs[0], 1),
		"status":    matchers.Term(requestPet.Status, "available|pending|sold"),
	}
	updatedPetMatcher := matchers.Map{
		"id":           matchers.Like(updatePet.ID),
		"name":         matchers.Like(updatePet.Name),
		"photoUrls":    matchers.ArrayMinLike(updatePet.PhotoURLs[0], 1),
		"status":       matchers.Term(updatePet.Status, "available|pending|sold"),
		"hairLengthCm": matchers.Like(updatePet.HairLengthCm),
	}
	searchPetMatcher := matchers.Map{
		"id":        matchers.Like(pacttest.SearchPetID),
		"name":      matchers.Like("Searchable Pact Pet"),
		"photoUrls": matchers.ArrayMinLike("https://example.pact/pets/searchable.png", 1),
		"status":    matchers.Term("available", "available|pending|sold"),
		"tags": matchers.ArrayMinLike(matchers.Map{
			"id":   matchers.Like(int64(1)),
			"name": matchers.Like("pact"),
		}, 1),
	}
	groomRequest := groomingPayload{InitialHairLengthCm: 3.5, TrimByCm: 1.0}
	formUpdatedMatcher := matchers.Map{
		"id":        matchers.Like(pacttest.ExistingPetID),
		"name":      matchers.Like("Fluffy Form Cat"),
		"photoUrls": matchers.ArrayMinLike(requestPet.PhotoURLs[0], 1),
		"status":    matchers.Term("pending", "available|pending|sold"),
	}
	groomedPetMatcher := matchers.Map{
		"id":           matchers.Like(pacttest.ExistingPetID),
		"name":         matchers.Like(requestPet.Name),
		"photoUrls":    matchers.ArrayMinLike(requestPet.PhotoURLs[0], 1),
		"status":       matchers.Term(requestPet.Status, "available|pending|sold"),
		"hairLengthCm": matchers.Like(groomRequest.InitialHairLengthCm - groomRequest.TrimByCm),
	}
	orderShipDate := time.Date(2024, 6, 12, 10, 0, 0, 0, time.UTC)
	orderRequest := orderPayload{
		ID:       pacttest.ExistingOrderID,
		PetID:    pacttest.ExistingPetID,
		Quantity: 2,
		ShipDate: orderShipDate,
		Status:   "approved",
		Complete: true,
	}
	orderMatcher := matchers.Map{
		"id":       matchers.Like(orderRequest.ID),
		"petId":    matchers.Like(orderRequest.PetID),
		"quantity": matchers.Like(orderRequest.Quantity),
		"shipDate": matchers.Regex(orderRequest.ShipDate.Format(time.RFC3339), "^\\d{4}-\\d{2}-\\d{2}T.*Z$"),
		"status":   matchers.Term(orderRequest.Status, "placed|approved|delivered"),
		"complete": matchers.Like(orderRequest.Complete),
	}
	inventoryMatcher := matchers.Map{
		"placed":    matchers.Like(3),
		"approved":  matchers.Like(2),
		"delivered": matchers.Like(2),
	}
	userRequest := userPayload{
		ID:         501,
		Username:   pacttest.UserPrimaryUsername,
		FirstName:  "Pact",
		LastName:   "User",
		Email:      "pact.user@example.com",
		Password:   pacttest.UserPassword,
		Phone:      "+1234567890",
		UserStatus: 1,
	}
	userMatcher := matchers.Map{
		"id":         matchers.Like(userRequest.ID),
		"username":   matchers.Like(userRequest.Username),
		"firstName":  matchers.Like(userRequest.FirstName),
		"lastName":   matchers.Like(userRequest.LastName),
		"email":      matchers.Like(userRequest.Email),
		"password":   matchers.Like(userRequest.Password),
		"phone":      matchers.Like(userRequest.Phone),
		"userStatus": matchers.Like(userRequest.UserStatus),
	}
	userBatchMatcher := matchers.ArrayMinLike(userMatcher, 2)
	userUpdateMatcher := matchers.Map{
		"id":         matchers.Like(userRequest.ID),
		"username":   matchers.Like(userRequest.Username),
		"firstName":  matchers.Like("Pact Updated"),
		"lastName":   matchers.Like("User"),
		"email":      matchers.Like("pact.user+updated@example.com"),
		"password":   matchers.Like(userRequest.Password),
		"phone":      matchers.Like("+1234509876"),
		"userStatus": matchers.Like(int32(3)),
	}
	jsonContentType := matchers.Regex("application/json; charset=utf-8", "application\\/json(?:;\\s?charset=utf-8)?")

	pact.AddInteraction().
		Given(pacttest.StatePetsBaseline).
		UponReceiving("a request to create a pet").
		WithRequest("POST", "/v2/pet", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(petBodyMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(petBodyMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StatePetExists).
		UponReceiving("a request to fetch an existing pet").
		WithRequest("GET", fmt.Sprintf("/v2/pet/%d", pacttest.ExistingPetID)).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(petBodyMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StatePetMissing).
		UponReceiving("a request for a missing pet").
		WithRequest("GET", fmt.Sprintf("/v2/pet/%d", pacttest.MissingPetID)).
		WillRespondWith(http.StatusNotFound, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", matchers.S("application/problem+json"))
			b.JSONBody(matchers.Map{
				"type":   matchers.S("/problems/not-found"),
				"title":  matchers.S("Resource Not Found"),
				"status": matchers.Like(http.StatusNotFound),
			})
		})

	pact.AddInteraction().
		Given(pacttest.StatePetExists).
		UponReceiving("a request to update an existing pet").
		WithRequest("PUT", "/v2/pet", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(updatedPetMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(updatedPetMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StatePetsSearch).
		UponReceiving("a request to find pets by status").
		WithRequest("GET", "/v2/pet/findByStatus", func(b *pactconsumer.V2RequestBuilder) {
			b.Query("status", matchers.S("available"), matchers.S("pending"))
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(matchers.ArrayMinLike(searchPetMatcher, 2))
		})

	pact.AddInteraction().
		Given(pacttest.StatePetsSearch).
		UponReceiving("a request to find pets by tags").
		WithRequest("GET", "/v2/pet/findByTags", func(b *pactconsumer.V2RequestBuilder) {
			b.Query("tags", matchers.S("pact"), matchers.S("featured"))
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(matchers.ArrayMinLike(searchPetMatcher, 1))
		})

	pact.AddInteraction().
		Given(pacttest.StatePetsSearch).
		UponReceiving("a request to delete a pet").
		WithRequest("DELETE", fmt.Sprintf("/v2/pet/%d", pacttest.SearchPetID)).
		WillRespondWith(http.StatusOK)

	pact.AddInteraction().
		Given(pacttest.StatePetExists).
		UponReceiving("a request to update a pet with form data").
		WithRequest("POST", fmt.Sprintf("/v2/pet/%d", pacttest.ExistingPetID), func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.Regex("application/x-www-form-urlencoded", "application\\/x-www-form-urlencoded.*"))
			b.Body("application/x-www-form-urlencoded", []byte("name=Fluffy+Form+Cat&status=pending"))
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(formUpdatedMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StatePetExists).
		UponReceiving("a request to groom a pet").
		WithRequest("POST", fmt.Sprintf("/v2/pet/%d/groom", pacttest.ExistingPetID), func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(matchers.Map{
				"initialHairLengthCm": matchers.Like(groomRequest.InitialHairLengthCm),
				"trimByCm":            matchers.Like(groomRequest.TrimByCm),
			})
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(groomedPetMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateOrdersBase).
		UponReceiving("a request to place an order").
		WithRequest("POST", "/v2/store/order", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(orderMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(orderMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateOrderExists).
		UponReceiving("a request to fetch an order").
		WithRequest("GET", fmt.Sprintf("/v2/store/order/%d", pacttest.ExistingOrderID)).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(orderMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateOrderExists).
		UponReceiving("a request to delete an order").
		WithRequest("DELETE", fmt.Sprintf("/v2/store/order/%d", pacttest.ExistingOrderID)).
		WillRespondWith(http.StatusOK)

	pact.AddInteraction().
		Given(pacttest.StateInventory).
		UponReceiving("a request to fetch inventory").
		WithRequest("GET", "/v2/store/inventory").
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(inventoryMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUsersBase).
		UponReceiving("a request to create a user").
		WithRequest("POST", "/v2/user", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(userMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(userMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUsersBase).
		UponReceiving("a request to create users with array").
		WithRequest("POST", "/v2/user/createWithArray", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(userBatchMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(userBatchMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUsersBase).
		UponReceiving("a request to create users with list").
		WithRequest("POST", "/v2/user/createWithList", func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(userBatchMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(userBatchMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUserExists).
		UponReceiving("a request to fetch a user").
		WithRequest("GET", fmt.Sprintf("/v2/user/%s", pacttest.UserPrimaryUsername)).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(userMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUserExists).
		UponReceiving("a request to update a user").
		WithRequest("PUT", fmt.Sprintf("/v2/user/%s", pacttest.UserPrimaryUsername), func(b *pactconsumer.V2RequestBuilder) {
			b.Header("Content-Type", matchers.S("application/json"))
			b.JSONBody(userUpdateMatcher)
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(userUpdateMatcher)
		})

	pact.AddInteraction().
		Given(pacttest.StateUserExists).
		UponReceiving("a request to delete a user").
		WithRequest("DELETE", fmt.Sprintf("/v2/user/%s", pacttest.UserPrimaryUsername)).
		WillRespondWith(http.StatusOK)

	pact.AddInteraction().
		Given(pacttest.StateUserExists).
		UponReceiving("a request to login a user").
		WithRequest("GET", "/v2/user/login", func(b *pactconsumer.V2RequestBuilder) {
			b.Query("username", matchers.S(pacttest.UserPrimaryUsername))
			b.Query("password", matchers.S(pacttest.UserPassword))
		}).
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(matchers.Like("logged in user session:" + pacttest.UserPrimaryUsername))
		})

	pact.AddInteraction().
		Given(pacttest.StateUsersBase).
		UponReceiving("a request to logout a user").
		WithRequest("GET", "/v2/user/logout").
		WillRespondWith(http.StatusOK, func(b *pactconsumer.V2ResponseBuilder) {
			b.Header("Content-Type", jsonContentType)
			b.JSONBody(matchers.Like("ok"))
		})

	err = pact.ExecuteTest(t, func(config pactconsumer.MockServerConfig) error {
		petClient := newPetClient(config)
		storeClient := newStoreClient(config)
		userClient := newUserClient(config)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		created, err := petClient.CreatePet(ctx, requestPet)
		if err != nil {
			return fmt.Errorf("create pet: %w", err)
		}
		if created == nil || created.ID == 0 {
			return fmt.Errorf("expected created pet ID to be set")
		}

		fetched, err := petClient.GetPet(ctx, pacttest.ExistingPetID)
		if err != nil {
			return fmt.Errorf("get pet: %w", err)
		}
		if fetched == nil || fetched.ID != pacttest.ExistingPetID {
			return fmt.Errorf("expected pet id %d, got %+v", pacttest.ExistingPetID, fetched)
		}

		if _, err := petClient.GetPet(ctx, pacttest.MissingPetID); err == nil {
			return fmt.Errorf("expected 404 for pet %d", pacttest.MissingPetID)
		} else if apiErr, ok := err.(apiError); ok && apiErr.Status() != http.StatusNotFound {
			return fmt.Errorf("expected 404, got %d", apiErr.Status())
		}

		updated, err := petClient.UpdatePet(ctx, updatePet)
		if err != nil {
			return fmt.Errorf("update pet: %w", err)
		}
		if updated == nil || updated.Status != updatePet.Status {
			return fmt.Errorf("expected updated status %s, got %+v", updatePet.Status, updated)
		}

		searchPets, err := petClient.FindByStatus(ctx, []string{"available", "pending"})
		if err != nil {
			return fmt.Errorf("find pets by status: %w", err)
		}
		if len(searchPets) == 0 {
			return fmt.Errorf("expected pets from status search")
		}

		taggedPets, err := petClient.FindByTags(ctx, []string{"pact", "featured"})
		if err != nil {
			return fmt.Errorf("find pets by tags: %w", err)
		}
		if len(taggedPets) == 0 {
			return fmt.Errorf("expected pets from tag search")
		}

		if err := petClient.DeletePet(ctx, pacttest.SearchPetID); err != nil {
			return fmt.Errorf("delete pet: %w", err)
		}

		formUpdated, err := petClient.UpdatePetWithForm(ctx, pacttest.ExistingPetID, "Fluffy Form Cat", "pending")
		if err != nil {
			return fmt.Errorf("update pet with form: %w", err)
		}
		if formUpdated == nil || formUpdated.Status == "" {
			return fmt.Errorf("expected form update response")
		}

		groomed, err := petClient.GroomPet(ctx, pacttest.ExistingPetID, groomRequest)
		if err != nil {
			return fmt.Errorf("groom pet: %w", err)
		}
		if groomed == nil || groomed.HairLengthCm == 0 {
			return fmt.Errorf("expected groomed hair length")
		}

		placedOrder, err := storeClient.PlaceOrder(ctx, orderRequest)
		if err != nil {
			return fmt.Errorf("place order: %w", err)
		}
		if placedOrder == nil || placedOrder.ID == 0 {
			return fmt.Errorf("expected placed order id")
		}

		order, err := storeClient.GetOrder(ctx, pacttest.ExistingOrderID)
		if err != nil {
			return fmt.Errorf("get order: %w", err)
		}
		if order == nil || order.ID != pacttest.ExistingOrderID {
			return fmt.Errorf("expected order %d, got %+v", pacttest.ExistingOrderID, order)
		}

		if err := storeClient.DeleteOrder(ctx, pacttest.ExistingOrderID); err != nil {
			return fmt.Errorf("delete order: %w", err)
		}

		inventory, err := storeClient.GetInventory(ctx)
		if err != nil {
			return fmt.Errorf("inventory: %w", err)
		}
		if len(inventory) == 0 {
			return fmt.Errorf("expected inventory entries")
		}

		createdUser, err := userClient.CreateUser(ctx, userRequest)
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if createdUser == nil || createdUser.Username == "" {
			return fmt.Errorf("expected created user")
		}

		userBatch := []userPayload{
			{
				ID:         601,
				Username:   pacttest.UserPrimaryUsername,
				FirstName:  "Pact",
				LastName:   "User",
				Email:      "pact.user@example.com",
				Password:   pacttest.UserPassword,
				Phone:      "+1234567890",
				UserStatus: 1,
			},
			{
				ID:         602,
				Username:   pacttest.UserSecondaryUsername,
				FirstName:  "Pact",
				LastName:   "Admin",
				Email:      "pact.admin@example.com",
				Password:   pacttest.UserPassword,
				Phone:      "+19876543210",
				UserStatus: 2,
			},
		}
		if _, err := userClient.CreateUsersWithArray(ctx, userBatch); err != nil {
			return fmt.Errorf("create users with array: %w", err)
		}
		if _, err := userClient.CreateUsersWithList(ctx, userBatch); err != nil {
			return fmt.Errorf("create users with list: %w", err)
		}

		user, err := userClient.GetUser(ctx, pacttest.UserPrimaryUsername)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if user == nil || user.Username != pacttest.UserPrimaryUsername {
			return fmt.Errorf("expected user %s", pacttest.UserPrimaryUsername)
		}

		updatedUser := userRequest
		updatedUser.FirstName = "Pact Updated"
		updatedUser.Email = "pact.user+updated@example.com"
		updatedUser.Phone = "+1234509876"
		updatedUser.UserStatus = 3
		if _, err := userClient.UpdateUser(ctx, pacttest.UserPrimaryUsername, updatedUser); err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		if err := userClient.DeleteUser(ctx, pacttest.UserPrimaryUsername); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}

		if _, err := userClient.Login(ctx, pacttest.UserPrimaryUsername, pacttest.UserPassword); err != nil {
			return fmt.Errorf("login user: %w", err)
		}

		if _, err := userClient.Logout(ctx); err != nil {
			return fmt.Errorf("logout user: %w", err)
		}

		return nil
	})
	require.NoError(t, err)
}

type petClient struct {
	baseURL    string
	httpClient *http.Client
}

func baseClient(config pactconsumer.MockServerConfig) (string, *http.Client) {
	host := config.Host
	if host == "" {
		host = "localhost"
	}
	transport := &http.Transport{TLSClientConfig: config.TLSConfig}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	return fmt.Sprintf("http://%s:%d", host, config.Port), client
}

func newPetClient(config pactconsumer.MockServerConfig) *petClient {
	baseURL, client := baseClient(config)
	return &petClient{
		baseURL:    baseURL,
		httpClient: client,
	}
}

func (c *petClient) CreatePet(ctx context.Context, pet petPayload) (*petPayload, error) {
	body, err := json.Marshal(pet)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/pet", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var payload petPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *petClient) GetPet(ctx context.Context, id int64) (*petPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v2/pet/%d", c.baseURL, id), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var payload petPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *petClient) UpdatePet(ctx context.Context, pet petPayload) (*petPayload, error) {
	body, err := json.Marshal(pet)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/v2/pet", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var payload petPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *petClient) FindByStatus(ctx context.Context, statuses []string) ([]petPayload, error) {
	values := url.Values{}
	for _, status := range statuses {
		values.Add("status", status)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/pet/findByStatus?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var pets []petPayload
	if err := json.NewDecoder(res.Body).Decode(&pets); err != nil {
		return nil, err
	}
	return pets, nil
}

func (c *petClient) FindByTags(ctx context.Context, tags []string) ([]petPayload, error) {
	values := url.Values{}
	for _, tag := range tags {
		values.Add("tags", tag)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/pet/findByTags?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var pets []petPayload
	if err := json.NewDecoder(res.Body).Decode(&pets); err != nil {
		return nil, err
	}
	return pets, nil
}

func (c *petClient) DeletePet(ctx context.Context, id int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/v2/pet/%d", c.baseURL, id), nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return decodeAPIError(res)
	}
	return nil
}

func (c *petClient) UpdatePetWithForm(ctx context.Context, id int64, name, status string) (*petPayload, error) {
	values := url.Values{}
	if strings.TrimSpace(name) != "" {
		values.Set("name", name)
	}
	if strings.TrimSpace(status) != "" {
		values.Set("status", status)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v2/pet/%d", c.baseURL, id), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload petPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *petClient) GroomPet(ctx context.Context, id int64, payload groomingPayload) (*petPayload, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v2/pet/%d/groom", c.baseURL, id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}

	var response petPayload
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

type storeClient struct {
	baseURL    string
	httpClient *http.Client
}

func newStoreClient(config pactconsumer.MockServerConfig) *storeClient {
	baseURL, client := baseClient(config)
	return &storeClient{baseURL: baseURL, httpClient: client}
}

func (c *storeClient) PlaceOrder(ctx context.Context, order orderPayload) (*orderPayload, error) {
	body, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/store/order", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload orderPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *storeClient) GetOrder(ctx context.Context, id int64) (*orderPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v2/store/order/%d", c.baseURL, id), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload orderPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *storeClient) DeleteOrder(ctx context.Context, id int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/v2/store/order/%d", c.baseURL, id), nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return decodeAPIError(res)
	}
	return nil
}

func (c *storeClient) GetInventory(ctx context.Context) (map[string]int32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/store/inventory", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload map[string]int32
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type userClient struct {
	baseURL    string
	httpClient *http.Client
}

func newUserClient(config pactconsumer.MockServerConfig) *userClient {
	baseURL, client := baseClient(config)
	return &userClient{baseURL: baseURL, httpClient: client}
}

func (c *userClient) CreateUser(ctx context.Context, user userPayload) (*userPayload, error) {
	body, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/user", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload userPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *userClient) CreateUsersWithArray(ctx context.Context, users []userPayload) ([]userPayload, error) {
	body, err := json.Marshal(users)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/user/createWithArray", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload []userPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *userClient) CreateUsersWithList(ctx context.Context, users []userPayload) ([]userPayload, error) {
	body, err := json.Marshal(users)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/user/createWithList", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload []userPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *userClient) GetUser(ctx context.Context, username string) (*userPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v2/user/%s", c.baseURL, username), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload userPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *userClient) UpdateUser(ctx context.Context, username string, user userPayload) (*userPayload, error) {
	body, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/v2/user/%s", c.baseURL, username), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, decodeAPIError(res)
	}
	var payload userPayload
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *userClient) DeleteUser(ctx context.Context, username string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/v2/user/%s", c.baseURL, username), nil)
	if err != nil {
		return err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return decodeAPIError(res)
	}
	return nil
}

func (c *userClient) Login(ctx context.Context, username, password string) (string, error) {
	values := url.Values{}
	values.Set("username", username)
	values.Set("password", password)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/user/login?"+values.Encode(), nil)
	if err != nil {
		return "", err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return "", decodeAPIError(res)
	}
	var token string
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return "", err
	}
	return token, nil
}

func (c *userClient) Logout(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/user/logout", nil)
	if err != nil {
		return "", err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return "", decodeAPIError(res)
	}
	var message string
	if err := json.NewDecoder(res.Body).Decode(&message); err != nil {
		return "", err
	}
	return message, nil
}

func decodeAPIError(res *http.Response) error {
	var problem problemDetail
	_ = json.NewDecoder(res.Body).Decode(&problem)
	status := problem.Status
	if status == 0 {
		status = res.StatusCode
	}
	return apiError{
		status: status,
		title:  problem.Title,
		detail: problem.Detail,
	}
}
