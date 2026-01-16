//go:build pact
// +build pact

package consumer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	pacttest "github.com/Apurer/go-gin-api-server/test/pact"

	pactconsumer "github.com/pact-foundation/pact-go/v2/consumer"
	pactlog "github.com/pact-foundation/pact-go/v2/log"
	"github.com/pact-foundation/pact-go/v2/matchers"
	"github.com/stretchr/testify/require"
)

type petPayload struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	PhotoURLs []string `json:"photoUrls"`
	Status    string   `json:"status"`
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
	petBodyMatcher := matchers.Map{
		"id":        matchers.Like(requestPet.ID),
		"name":      matchers.Like(requestPet.Name),
		"photoUrls": matchers.ArrayMinLike(requestPet.PhotoURLs[0], 1),
		"status":    matchers.Term(requestPet.Status, "available|pending|sold"),
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

	err = pact.ExecuteTest(t, func(config pactconsumer.MockServerConfig) error {
		client := newPetClient(config)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		created, err := client.CreatePet(ctx, requestPet)
		if err != nil {
			return fmt.Errorf("create pet: %w", err)
		}
		if created == nil || created.ID == 0 {
			return fmt.Errorf("expected created pet ID to be set")
		}

		fetched, err := client.GetPet(ctx, pacttest.ExistingPetID)
		if err != nil {
			return fmt.Errorf("get pet: %w", err)
		}
		if fetched == nil || fetched.ID != pacttest.ExistingPetID {
			return fmt.Errorf("expected pet id %d, got %+v", pacttest.ExistingPetID, fetched)
		}

		if _, err := client.GetPet(ctx, pacttest.MissingPetID); err == nil {
			return fmt.Errorf("expected 404 for pet %d", pacttest.MissingPetID)
		} else if apiErr, ok := err.(apiError); ok && apiErr.Status() != http.StatusNotFound {
			return fmt.Errorf("expected 404, got %d", apiErr.Status())
		}

		return nil
	})
	require.NoError(t, err)
}

type petClient struct {
	baseURL    string
	httpClient *http.Client
}

func newPetClient(config pactconsumer.MockServerConfig) *petClient {
	host := config.Host
	if host == "" {
		host = "localhost"
	}
	transport := &http.Transport{TLSClientConfig: config.TLSConfig}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	return &petClient{
		baseURL:    fmt.Sprintf("http://%s:%d", host, config.Port),
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
