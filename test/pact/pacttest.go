//go:build pact
// +build pact

package pacttest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	ProviderName = "petstore-api"
	ConsumerName = "pet-portal"

	StatePetsBaseline = "pets baseline"
	StatePetExists    = "pet with id 101 exists"
	StatePetMissing   = "no pet with id 404"
	StatePetsSearch   = "pets exist for search queries"
	StateOrdersBase   = "store orders baseline"
	StateOrderExists  = "order with id 301 exists"
	StateInventory    = "store inventory seeded"
	StateUsersBase    = "users baseline"
	StateUserExists   = "user pact-user exists"
)

const (
	ExistingPetID int64 = 101
	MissingPetID  int64 = 404
	SearchPetID   int64 = 202

	ExistingOrderID int64 = 301
	MissingOrderID  int64 = 999

	UserPrimaryUsername   = "pact-user"
	UserSecondaryUsername = "pact-admin"
	MissingUsername       = "ghost-user"
	UserPassword          = "pact-pass"
)

const (
	examplePhotoURL = "https://example.pact/pets/fluffy.png"
	examplePetName  = "Fluffy Pact Cat"
	exampleShipDate = "2024-06-12T10:00:00Z"
)

// PactDir returns the workspace-level directory for generated pact files.
func PactDir(t testing.TB) string {
	t.Helper()
	dir := filepath.Join(projectRoot(t), "pacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create pact dir: %v", err)
	}
	return dir
}

// PactFile returns the canonical pact file path for the pet portal consumer.
func PactFile(t testing.TB) string {
	t.Helper()
	return filepath.Join(PactDir(t), ConsumerName+"-"+ProviderName+".json")
}

// LogDir returns the log output directory for pact-go.
func LogDir(t testing.TB) string {
	t.Helper()
	dir := filepath.Join(projectRoot(t), "bin", "pact-logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create pact log dir: %v", err)
	}
	return dir
}

// ExamplePetPayload provides stable test data for pact interactions.
func ExamplePetPayload() map[string]any {
	return map[string]any{
		"id":        ExistingPetID,
		"name":      examplePetName,
		"photoUrls": []string{examplePhotoURL},
		"status":    "available",
	}
}

// ExampleOrderPayload provides stable test data for order interactions.
func ExampleOrderPayload() map[string]any {
	return map[string]any{
		"id":       ExistingOrderID,
		"petId":    ExistingPetID,
		"quantity": 2,
		"shipDate": exampleShipDate,
		"status":   "approved",
		"complete": true,
	}
}

// ExampleUserPayload provides stable test data for user interactions.
func ExampleUserPayload() map[string]any {
	return map[string]any{
		"id":         501,
		"username":   UserPrimaryUsername,
		"firstName":  "Pact",
		"lastName":   "User",
		"email":      "pact.user@example.com",
		"password":   UserPassword,
		"phone":      "+1234567890",
		"userStatus": 1,
	}
}

// ExampleSecondaryUserPayload adds a second user for batch flows.
func ExampleSecondaryUserPayload() map[string]any {
	return map[string]any{
		"id":         502,
		"username":   UserSecondaryUsername,
		"firstName":  "Pact",
		"lastName":   "Admin",
		"email":      "pact.admin@example.com",
		"password":   UserPassword,
		"phone":      "+19876543210",
		"userStatus": 2,
	}
}

// ExampleUserBatch returns a slice with both example users.
func ExampleUserBatch() []map[string]any {
	return []map[string]any{ExampleUserPayload(), ExampleSecondaryUserPayload()}
}

// projectRoot walks up from this file to the workspace root.
func projectRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller for pact paths")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
