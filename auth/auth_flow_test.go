package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInteractiveAuth_MissingOAuthEnv(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	store := &LocalDirectoryCredentialStore{Dir: t.TempDir()}
	_, _, err := RunInteractiveAuth(context.Background(), "user@example.com", store, nil)
	if err == nil {
		t.Fatal("expected error when OAuth env vars are missing")
	}
	if !strings.Contains(err.Error(), "GOOGLE_OAUTH_CLIENT_ID") {
		t.Errorf("expected error mentioning GOOGLE_OAUTH_CLIENT_ID, got %q", err.Error())
	}
}

func TestCredentialPath(t *testing.T) {
	dir := t.TempDir()
	store := &LocalDirectoryCredentialStore{Dir: dir}
	got := store.CredentialPath("user@example.com")
	want := filepath.Join(dir, "user@example.com.json")
	if got != want {
		t.Errorf("CredentialPath = %q, want %q", got, want)
	}
}

func TestAuthStartMessageMentionsCLI(t *testing.T) {
	msg := authStartMessage("Gmail", "user@example.com", "https://example.com/auth")
	if !strings.Contains(msg, "google-mcp auth") {
		t.Errorf("expected CLI auth hint in message, got %q", msg)
	}
}
