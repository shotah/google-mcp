package google

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/shotah/google-mcp/auth"
)

// writeSampleCredential writes a valid credential JSON file for testing.
func writeSampleCredential(t *testing.T, dir, email string) {
	t.Helper()
	cred := map[string]any{
		"token":         "access-token-123",
		"refresh_token": "refresh-token-456",
		"token_uri":     "https://oauth2.googleapis.com/token",
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
		"scopes":        []string{"https://www.googleapis.com/auth/gmail.modify"},
		"expiry":        time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, email+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGetAuthenticatedClient_Success(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email := "user@example.com"
	writeSampleCredential(t, dir, email)

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	client, err := cache.GetAuthenticatedClient(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestGetAuthenticatedClient_MissingCredentials(t *testing.T) {
	dir := t.TempDir()
	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	_, err := cache.GetAuthenticatedClient(context.Background(), "nobody@example.com")
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	if want := "no credentials found for nobody@example.com; run: google-mcp auth"; err.Error() != want {
		t.Errorf("got error %q, want %q", err.Error(), want)
	}
}

func TestGetAuthenticatedClient_CachesClient(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email := "cached@example.com"
	writeSampleCredential(t, dir, email)

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	client1, err := cache.GetAuthenticatedClient(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client2, err := cache.GetAuthenticatedClient(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same pointer means the client was cached.
	if client1 != client2 {
		t.Error("expected same client instance from cache")
	}
}

func TestGetAuthenticatedClient_Invalidate(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email := "invalidate@example.com"
	writeSampleCredential(t, dir, email)

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	client1, err := cache.GetAuthenticatedClient(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cache.Invalidate(email)

	client2, err := cache.GetAuthenticatedClient(context.Background(), email)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After invalidation, a new client should be created.
	if client1 == client2 {
		t.Error("expected different client instance after invalidation")
	}
}

func TestGetAuthenticatedClient_ConcurrentAccess(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email := "concurrent@example.com"
	writeSampleCredential(t, dir, email)

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for range 10 {
		wg.Go(func() {
			client, err := cache.GetAuthenticatedClient(context.Background(), email)
			if err != nil {
				errs <- err
				return
			}
			if client == nil {
				errs <- errors.New("got nil client")
			}
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestGetAuthenticatedClient_MultipleUsers(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email1 := "alice@example.com"
	email2 := "bob@example.com"
	writeSampleCredential(t, dir, email1)
	writeSampleCredential(t, dir, email2)

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	client1, err := cache.GetAuthenticatedClient(context.Background(), email1)
	if err != nil {
		t.Fatalf("unexpected error for %s: %v", email1, err)
	}

	client2, err := cache.GetAuthenticatedClient(context.Background(), email2)
	if err != nil {
		t.Fatalf("unexpected error for %s: %v", email2, err)
	}

	// Different users should get different clients.
	if client1 == client2 {
		t.Error("expected different clients for different users")
	}
}

func TestInvalidate_NonexistentUser(t *testing.T) {
	dir := t.TempDir()
	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	// Should not panic.
	cache.Invalidate("nonexistent@example.com")
}

func TestDefaultClientCache(t *testing.T) {
	a := DefaultClientCache()
	b := DefaultClientCache()
	if a == nil || a != b {
		t.Fatalf("DefaultClientCache should return the same non-nil singleton")
	}
}
