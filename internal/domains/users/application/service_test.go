package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"
	"github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/ports"
)

type fakeUserRepo struct {
	users map[string]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*domain.User{}}
}

func (f *fakeUserRepo) Save(_ context.Context, user *domain.User) (*domain.User, error) {
	copy := *user
	f.users[user.Username] = &copy
	return &copy, nil
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	if u, ok := f.users[username]; ok {
		copy := *u
		return &copy, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeUserRepo) Delete(_ context.Context, username string) error {
	if _, ok := f.users[username]; !ok {
		return ports.ErrNotFound
	}
	delete(f.users, username)
	return nil
}

func (f *fakeUserRepo) List(_ context.Context) ([]*domain.User, error) {
	var list []*domain.User
	for _, u := range f.users {
		copy := *u
		list = append(list, &copy)
	}
	return list, nil
}

type fakeSessionStore struct {
	sessions map[string]string
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{sessions: map[string]string{}}
}

func (f *fakeSessionStore) Save(_ context.Context, username, token string) error {
	f.sessions[username] = token
	return nil
}

func (f *fakeSessionStore) Delete(_ context.Context, username string) error {
	delete(f.sessions, username)
	return nil
}

func TestCreateAndLoginUser(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionStore()
	svc := NewService(repo, sessions)

	user, err := domain.NewUser(1, "alice", "secret")
	require.NoError(t, err)
	created, err := svc.CreateUser(context.Background(), user)
	require.NoError(t, err)
	require.Equal(t, "alice", created.Username)

	token, err := svc.Login(context.Background(), "alice", "secret")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.NotEmpty(t, sessions.sessions["alice"])
}

func TestLogin_InvalidCredentials(t *testing.T) {
	repo := newFakeUserRepo()
	sessions := newFakeSessionStore()
	svc := NewService(repo, sessions)

	_, err := svc.Login(context.Background(), "missing", "secret")
	require.Error(t, err)
}
