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
)

const (
	ExistingPetID int64 = 101
	MissingPetID  int64 = 404
)

const (
	examplePhotoURL = "https://example.pact/pets/fluffy.png"
	examplePetName  = "Fluffy Pact Cat"
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

// projectRoot walks up from this file to the workspace root.
func projectRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller for pact paths")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
