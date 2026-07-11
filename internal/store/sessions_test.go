package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func tokenHash(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func TestSessions(t *testing.T) {
	pool := testPool(t)
	users := NewUsers(pool)
	sessions := NewSessions(pool)
	ctx := context.Background()

	user, err := users.Create(ctx, CreateUserParams{
		Email: "session@example.com", GoogleSub: strPtr("google-sub-session"),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Run("create and resolve", func(t *testing.T) {
		hash := tokenHash("token-1")
		if err := sessions.Create(ctx, hash, user.ID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := sessions.UserByToken(ctx, hash)
		if err != nil {
			t.Fatalf("UserByToken: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("resolved wrong user: %+v", got)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		if _, err := sessions.UserByToken(ctx, tokenHash("never-issued")); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		hash := tokenHash("token-expired")
		if err := sessions.Create(ctx, hash, user.ID, time.Now().Add(-time.Minute)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := sessions.UserByToken(ctx, hash); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		hash := tokenHash("token-deleted")
		if err := sessions.Create(ctx, hash, user.ID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := sessions.Delete(ctx, hash); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := sessions.UserByToken(ctx, hash); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound after delete", err)
		}
	})

	t.Run("delete for user removes all sessions", func(t *testing.T) {
		for _, tok := range []string{"multi-1", "multi-2"} {
			if err := sessions.Create(ctx, tokenHash(tok), user.ID, time.Now().Add(time.Hour)); err != nil {
				t.Fatalf("Create: %v", err)
			}
		}
		if err := sessions.DeleteForUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteForUser: %v", err)
		}
		for _, tok := range []string{"multi-1", "multi-2"} {
			if _, err := sessions.UserByToken(ctx, tokenHash(tok)); !errors.Is(err, ErrNotFound) {
				t.Fatalf("session %s survived DeleteForUser", tok)
			}
		}
	})

	t.Run("delete expired", func(t *testing.T) {
		if err := sessions.Create(ctx, tokenHash("live"), user.ID, time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := sessions.Create(ctx, tokenHash("stale"), user.ID, time.Now().Add(-time.Hour)); err != nil {
			t.Fatalf("Create: %v", err)
		}
		n, err := sessions.DeleteExpired(ctx)
		if err != nil {
			t.Fatalf("DeleteExpired: %v", err)
		}
		if n != 1 {
			t.Fatalf("DeleteExpired removed %d sessions, want 1", n)
		}
		if _, err := sessions.UserByToken(ctx, tokenHash("live")); err != nil {
			t.Fatalf("live session should survive: %v", err)
		}
	})
}
