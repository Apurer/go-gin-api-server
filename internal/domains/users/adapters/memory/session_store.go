package memory

import (
	"context"
	"sync"
)

// SessionStore is an in-memory SessionStore implementation.
type SessionStore struct {
	session sync.Map
}

func NewSessionStore() *SessionStore {
	return &SessionStore{}
}

func (s *SessionStore) Save(_ context.Context, username, token string) error {
	s.session.Store(username, token)
	return nil
}

func (s *SessionStore) Delete(_ context.Context, username string) error {
	s.session.Delete(username)
	return nil
}
