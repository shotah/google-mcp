package google

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestGetAuthenticatedClient_CanceledCtxDoesNotPoisonRefresh(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	refreshHits := 0
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-access","expires_in":3600,"token_type":"Bearer"}`))
	}))
	t.Cleanup(tokenSrv.Close)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer refreshed-access" {
			http.Error(w, "bad auth "+got, http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(apiSrv.Close)

	dir := t.TempDir()
	email := "refresh@example.com"
	cred := map[string]any{
		"token":         "expired-access",
		"refresh_token": "refresh-token-456",
		"token_uri":     tokenSrv.URL,
		"client_id":     "test-client-id",
		"client_secret": "test-client-secret",
		"scopes":        []string{"https://www.googleapis.com/auth/gmail.modify"},
		// Already expired → first API call must refresh.
		"expiry": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, email+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	store := &auth.LocalDirectoryCredentialStore{Dir: dir}
	cache := NewClientCache(store)

	// Simulate MCP tool ctx that is canceled after the client is cached
	// (turn end / interrupt). Refresh must not use that canceled ctx.
	ctx, cancel := context.WithCancel(context.Background())
	client, err := cache.GetAuthenticatedClient(ctx, email)
	if err != nil {
		t.Fatalf("GetAuthenticatedClient: %v", err)
	}
	cancel()

	req, err := http.NewRequest(http.MethodGet, apiSrv.URL, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("API call after canceled tool ctx: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if refreshHits < 1 {
		t.Fatal("expected token refresh hit")
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
