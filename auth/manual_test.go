package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateAuthURL_Success(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "")

	dir := t.TempDir()
	authURL, err := GenerateAuthURL(dir)
	if err != nil {
		t.Fatalf("GenerateAuthURL: %v", err)
	}

	if !strings.Contains(authURL, "accounts.google.com") {
		t.Errorf("URL missing Google auth domain: %s", authURL)
	}
	if !strings.Contains(authURL, "code_challenge=") {
		t.Errorf("URL missing PKCE code_challenge: %s", authURL)
	}
	if !strings.Contains(authURL, "code_challenge_method=S256") {
		t.Errorf("URL missing S256 method: %s", authURL)
	}
	if !strings.Contains(authURL, "access_type=offline") {
		t.Errorf("URL missing access_type=offline: %s", authURL)
	}
	if !strings.Contains(authURL, "prompt=consent") {
		t.Errorf("URL missing prompt=consent: %s", authURL)
	}
	if !strings.Contains(authURL, "state=") {
		t.Errorf("URL missing state: %s", authURL)
	}

	defaultURI := "https%3A%2F%2Fshotah.github.io%2Foauth-catch%2F"
	if !strings.Contains(authURL, defaultURI) {
		t.Errorf("URL missing default redirect URI: %s", authURL)
	}

	ps, err := LoadPendingSession(dir)
	if err != nil {
		t.Fatalf("LoadPendingSession after generate: %v", err)
	}
	if ps.Verifier == "" {
		t.Error("pending session verifier is empty")
	}
	if ps.State == "" {
		t.Error("pending session state is empty")
	}
	if ps.RedirectURI != defaultRedirectURI {
		t.Errorf("redirect_uri = %q, want %q", ps.RedirectURI, defaultRedirectURI)
	}
	if ps.ExpiresAt.Before(time.Now()) {
		t.Error("pending session already expired")
	}
}

func TestGenerateAuthURL_CustomRedirectURI(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-client-secret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "http://localhost:9999/callback")

	dir := t.TempDir()
	authURL, err := GenerateAuthURL(dir)
	if err != nil {
		t.Fatalf("GenerateAuthURL: %v", err)
	}
	if !strings.Contains(authURL, "localhost%3A9999") {
		t.Errorf("URL should contain custom redirect URI: %s", authURL)
	}

	ps, err := LoadPendingSession(dir)
	if err != nil {
		t.Fatalf("LoadPendingSession: %v", err)
	}
	if ps.RedirectURI != "http://localhost:9999/callback" {
		t.Errorf("redirect_uri = %q", ps.RedirectURI)
	}
}

func TestGenerateAuthURL_MissingCredentials(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	_, err := GenerateAuthURL(t.TempDir())
	if err == nil {
		t.Fatal("expected error when OAuth env vars are missing")
	}
	if !strings.Contains(err.Error(), "GOOGLE_OAUTH_CLIENT_ID") {
		t.Errorf("error should mention missing env vars: %v", err)
	}
}

func TestManualRedirectURI_Default(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "")
	if got := ManualRedirectURI(); got != defaultRedirectURI {
		t.Errorf("ManualRedirectURI() = %q, want %q", got, defaultRedirectURI)
	}
}

func TestManualRedirectURI_EnvOverride(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "http://custom/cb")
	if got := ManualRedirectURI(); got != "http://custom/cb" {
		t.Errorf("ManualRedirectURI() = %q, want http://custom/cb", got)
	}
}

func TestExchangeAuthCode_HappyPath(t *testing.T) {
	// Fake token endpoint
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "ya29.fake-access-token",
			"refresh_token": "1//fake-refresh",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()

	// Fake userinfo endpoint
	userinfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"email": "user@example.com",
		})
	}))
	defer userinfoSrv.Close()

	origFetch := fetchUserEmailFn
	fetchUserEmailFn = func(tok *tokenForEmail) (string, error) {
		return "user@example.com", nil
	}
	defer func() { fetchUserEmailFn = origFetch }()

	dir := t.TempDir()
	credDir := filepath.Join(dir, "creds")
	store := &LocalDirectoryCredentialStore{Dir: credDir}

	now := time.Now()
	ps := &PendingSession{
		Verifier:    "test-verifier",
		State:       "test-state",
		RedirectURI: tokenSrv.URL + "/callback",
		CreatedAt:   now,
		ExpiresAt:   now.Add(pendingTTL),
	}
	if err := SavePendingSession(credDir, ps); err != nil {
		t.Fatalf("save pending: %v", err)
	}

	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-cid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-csecret")

	email, path, err := ExchangeAuthCodeWithTokenURL(
		context.Background(), credDir, "fake-auth-code", store, tokenSrv.URL,
	)
	if err != nil {
		t.Fatalf("ExchangeAuthCode: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", email)
	}
	if !strings.HasSuffix(path, "user@example.com.json") {
		t.Errorf("path = %q, expected to end with user@example.com.json", path)
	}

	cred, err := store.GetCredential("user@example.com")
	if err != nil {
		t.Fatalf("GetCredential after exchange: %v", err)
	}
	if cred == nil {
		t.Fatal("credential should exist after exchange")
	}
	if cred.Token.RefreshToken != "1//fake-refresh" {
		t.Errorf("RefreshToken = %q", cred.Token.RefreshToken)
	}

	if _, statErr := os.Stat(filepath.Join(credDir, pendingFileName)); !os.IsNotExist(statErr) {
		t.Error("pending file should be deleted after successful exchange")
	}
}

func TestExchangeAuthCode_NoPending(t *testing.T) {
	dir := t.TempDir()
	store := &LocalDirectoryCredentialStore{Dir: dir}

	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "cid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "csecret")

	_, _, err := ExchangeAuthCode(context.Background(), dir, "code", store)
	if err == nil {
		t.Fatal("expected error when no pending session exists")
	}
	if !strings.Contains(err.Error(), "no pending auth session") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExchangeAuthCode_ExpiredPending(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().Add(-20 * time.Minute)
	ps := &PendingSession{
		Verifier:    "v",
		State:       "s",
		RedirectURI: "https://example.com",
		CreatedAt:   past,
		ExpiresAt:   past.Add(pendingTTL),
	}
	if err := SavePendingSession(dir, ps); err != nil {
		t.Fatalf("save: %v", err)
	}

	store := &LocalDirectoryCredentialStore{Dir: dir}
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "cid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "csecret")

	_, _, err := ExchangeAuthCode(context.Background(), dir, "code", store)
	if err == nil {
		t.Fatal("expected error for expired pending session")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExchangeAuthCode_MissingCredentials(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	ps := &PendingSession{
		Verifier:    "v",
		State:       "s",
		RedirectURI: "https://example.com",
		CreatedAt:   now,
		ExpiresAt:   now.Add(pendingTTL),
	}
	if err := SavePendingSession(dir, ps); err != nil {
		t.Fatalf("save: %v", err)
	}

	store := &LocalDirectoryCredentialStore{Dir: dir}
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	_, _, err := ExchangeAuthCode(context.Background(), dir, "code", store)
	if err == nil {
		t.Fatal("expected error when OAuth env vars are missing")
	}
	if !strings.Contains(err.Error(), "GOOGLE_OAUTH_CLIENT_ID") {
		t.Errorf("error should mention missing env vars: %v", err)
	}
}
