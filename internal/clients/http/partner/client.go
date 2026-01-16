package partner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client wraps the generated PartnerAPIClient with a simplified SyncPet helper.
type Client struct {
	api *ClientWithResponses
}

// SyncOption configures SyncPet behavior.
type SyncOption func(*syncOptions)

// syncOptions holds optional request parameters.
type syncOptions struct {
	idempotencyKey string
}

// WithIdempotencyKey sets the Idempotency-Key header for the request.
func WithIdempotencyKey(key string) SyncOption {
	return func(opts *syncOptions) {
		opts.idempotencyKey = strings.TrimSpace(key)
	}
}

// NewPartnerClient instantiates the partner client with sane defaults.
func NewPartnerClient(baseURL string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("partner base URL is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	api, err := NewClientWithResponses(baseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("build partner client: %w", err)
	}
	return &Client{api: api}, nil
}

// SyncPet pushes the payload to the partner API.
func (c *Client) SyncPet(ctx context.Context, payload PetPayload, optFns ...SyncOption) error {
	if c == nil || c.api == nil {
		return errors.New("partner client not configured")
	}
	reference := strings.TrimSpace(payload.Reference)
	if reference == "" {
		return errors.New("partner reference is required")
	}
	var opts syncOptions
	for _, fn := range optFns {
		if fn != nil {
			fn(&opts)
		}
	}
	var params *SyncPetParams
	if opts.idempotencyKey != "" {
		params = &SyncPetParams{IdempotencyKey: &opts.idempotencyKey}
	}
	resp, err := c.api.SyncPetWithResponse(ctx, reference, params, payload)
	if err != nil {
		return fmt.Errorf("call partner API: %w", err)
	}
	if resp == nil || resp.StatusCode() == 0 {
		return errors.New("partner API returned an empty response")
	}
	status := resp.StatusCode()
	switch {
	case status == http.StatusOK:
		return nil
	case status == http.StatusConflict:
		return fmt.Errorf("partner API idempotency conflict: %s", errorMessage(resp.JSON409, resp.Status()))
	case status >= http.StatusBadRequest:
		return fmt.Errorf("partner API error: %s", errorMessage(firstError(resp), resp.Status()))
	default:
		return fmt.Errorf("partner API unexpected status: %s", resp.Status())
	}
}

func firstError(resp *SyncPetResponse) *Error {
	if resp == nil {
		return nil
	}
	if resp.JSON409 != nil {
		return resp.JSON409
	}
	if resp.JSON4XX != nil {
		return resp.JSON4XX
	}
	if resp.JSON5XX != nil {
		return resp.JSON5XX
	}
	return nil
}

func errorMessage(body *Error, fallback string) string {
	if body == nil {
		return fallback
	}
	if body.Message != nil {
		if msg := strings.TrimSpace(*body.Message); msg != "" {
			return msg
		}
	}
	if body.Status != nil {
		if msg := strings.TrimSpace(*body.Status); msg != "" {
			return msg
		}
	}
	return fallback
}
