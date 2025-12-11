package ports

import "context"

// SessionStore abstracts session/token persistence.
type SessionStore interface {
	Save(ctx context.Context, username, token string) error
	Delete(ctx context.Context, username string) error
}

// NoopSessionStore is a safe default when callers do not need session persistence.
var NoopSessionStore SessionStore = noopSessionStore{}

type noopSessionStore struct{}

func (noopSessionStore) Save(_ context.Context, _ string, _ string) error  { return nil }
func (noopSessionStore) Delete(_ context.Context, _ string) error           { return nil }
