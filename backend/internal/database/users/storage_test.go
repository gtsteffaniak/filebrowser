package users

import (
	"fmt"
	"strings"
	"testing"
)

// Interface is implemented by storage
var _ Store = &Storage{}

type mockStorageBackend struct {
	users map[uint64]*User
}

func (m *mockStorageBackend) GetBy(id uint64) (*User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %d", id)
	}
	return user, nil
}

func (m *mockStorageBackend) Gets() ([]*User, error) {
	out := make([]*User, 0, len(m.users))
	for _, user := range m.users {
		out = append(out, user)
	}
	return out, nil
}

func (m *mockStorageBackend) Save(u *User, _changePass, _disableScopeChange bool) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockStorageBackend) Update(u *User, _adminActor bool, _fields ...string) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockStorageBackend) DeleteByID(id uint64) error {
	delete(m.users, id)
	return nil
}

func newStorageWithUser(t *testing.T, tokens map[string]AuthToken) (*Storage, uint64) {
	t.Helper()
	const userID uint64 = 42
	backend := &mockStorageBackend{
		users: map[uint64]*User{
			userID: {
				ID: userID,
				FrontendUser: FrontendUser{
					Username: "alice",
				},
				Tokens: tokens,
			},
		},
	}
	return NewStorage(backend), userID
}

func TestAddApiTokenSuccess(t *testing.T) {
	storage, userID := newStorageWithUser(t, nil)

	err := storage.AddApiToken(userID, "ci-key", "jwt-abc-123", AuthToken{})
	if err != nil {
		t.Fatalf("AddApiToken: %v", err)
	}

	user, err := storage.Get(userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	name, ok := TokenNameByRaw(user.Tokens, "jwt-abc-123")
	if !ok || name != "ci-key" {
		t.Fatalf("TokenNameByRaw: got (%q, %v)", name, ok)
	}
}

func TestAddApiTokenRejectsDuplicateName(t *testing.T) {
	tokens := make(map[string]AuthToken)
	StoreToken(tokens, AuthToken{Name: "ci-key", Token: "jwt-existing"})
	storage, userID := newStorageWithUser(t, tokens)

	err := storage.AddApiToken(userID, "ci-key", "jwt-new", AuthToken{})
	if err == nil {
		t.Fatal("expected name collision error")
	}
	if !strings.Contains(err.Error(), `token with name "ci-key" already exists`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddApiTokenRejectsTokenValueCollision(t *testing.T) {
	tokens := make(map[string]AuthToken)
	StoreToken(tokens, AuthToken{Name: "existing-key", Token: "jwt-shared"})
	storage, userID := newStorageWithUser(t, tokens)

	err := storage.AddApiToken(userID, "new-key", "jwt-shared", AuthToken{})
	if err == nil {
		t.Fatal("expected token value collision error")
	}
	if !strings.Contains(err.Error(), "token value collides with an existing token key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
