package partner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PetPayload represents the simplified DTO exchanged with the partner API.
type PetPayload struct {
	Reference    string            `json:"reference"`
	Title        string            `json:"title"`
	Photos       []string          `json:"photos"`
	Labels       map[string]string `json:"labels"`
	Availability string            `json:"availability"`
}

// Client provides a tiny demo HTTP client capable of syncing a pet payload.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient instantiates the partner client with sane defaults.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: httpClient,
	}
}

// SyncPet pushes the payload to the partner API.
func (c *Client) SyncPet(ctx context.Context, payload PetPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serialize partner payload: %w", err)
	}
	endpoint := fmt.Sprintf("%s/pets/%s", c.BaseURL, payload.Reference)
	// Partner expects POST and does not support idempotency keys; retries must be controlled by caller.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build partner request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("call partner API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("partner API error: %s - %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}
