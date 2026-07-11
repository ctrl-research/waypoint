package store

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func strPtr(s string) *string { return &s }

func TestUsers(t *testing.T) {
	pool := testPool(t)
	users := NewUsers(pool)
	ctx := context.Background()

	t.Run("create and fetch google user", func(t *testing.T) {
		created, err := users.Create(ctx, CreateUserParams{
			Email:       "ada@example.com",
			DisplayName: "Ada Lovelace",
			AvatarURL:   strPtr("https://example.com/ada.png"),
			GoogleSub:   strPtr("google-sub-ada"),
			IsAdmin:     true,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if created.ID == uuid.Nil {
			t.Fatal("expected generated ID")
		}

		for name, fetch := range map[string]func() (User, error){
			"ByID":        func() (User, error) { return users.ByID(ctx, created.ID) },
			"ByEmail":     func() (User, error) { return users.ByEmail(ctx, "ada@example.com") },
			"ByGoogleSub": func() (User, error) { return users.ByGoogleSub(ctx, "google-sub-ada") },
		} {
			got, err := fetch()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got.ID != created.ID || got.Email != "ada@example.com" || !got.IsAdmin {
				t.Fatalf("%s returned wrong user: %+v", name, got)
			}
		}
	})

	t.Run("email lookup is case-insensitive", func(t *testing.T) {
		if _, err := users.Create(ctx, CreateUserParams{
			Email: "Case@Example.com", GoogleSub: strPtr("google-sub-case"),
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := users.ByEmail(ctx, "case@example.com"); err != nil {
			t.Fatalf("ByEmail lowercase: %v", err)
		}
	})

	t.Run("duplicate email rejected", func(t *testing.T) {
		if _, err := users.Create(ctx, CreateUserParams{
			Email: "ADA@example.com", GoogleSub: strPtr("google-sub-dup"),
		}); err == nil {
			t.Fatal("expected unique violation for duplicate email")
		}
	})

	t.Run("user without any credential rejected", func(t *testing.T) {
		if _, err := users.Create(ctx, CreateUserParams{Email: "nocred@example.com"}); err == nil {
			t.Fatal("expected check violation: no google_sub and no password_hash")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := users.ByEmail(ctx, "ghost@example.com"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if _, err := users.ByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("update profile", func(t *testing.T) {
		u, err := users.Create(ctx, CreateUserParams{
			Email: "update@example.com", GoogleSub: strPtr("google-sub-update"),
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := users.UpdateProfile(ctx, u.ID, "New Name", strPtr("https://example.com/new.png"))
		if err != nil {
			t.Fatalf("UpdateProfile: %v", err)
		}
		if got.DisplayName != "New Name" || got.AvatarURL == nil || *got.AvatarURL != "https://example.com/new.png" {
			t.Fatalf("profile not updated: %+v", got)
		}
	})

	t.Run("count", func(t *testing.T) {
		n, err := users.Count(ctx)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if n == 0 {
			t.Fatal("expected nonzero user count")
		}
	})
}
