package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestGetCredential_ParsesAllFields(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	dir := t.TempDir()
	email := "test@example.com"
	cred := credentialJSON{
		Token:        "ya29.access-token",
		RefreshToken: "1//refresh-token",
		TokenURI:     "https://oauth2.googleapis.com/token",
		ClientID:     "file-client-id.apps.googleusercontent.com",
		ClientSecret: "file-client-secret",
		Scopes:       []string{"https://www.googleapis.com/auth/gmail.modify", "https://www.googleapis.com/auth/drive"},
		Expiry:       "2026-03-01T12:00:00Z",
	}
	writeCred(t, dir, email, cred)

	store := &LocalDirectoryCredentialStore{Dir: dir}
	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if got == nil {
		t.Fatal("GetCredential returned nil")
		return
	}

	if got.Token.AccessToken != "ya29.access-token" {
		t.Errorf("AccessToken = %q, want %q", got.Token.AccessToken, "ya29.access-token")
	}
	if got.Token.RefreshToken != "1//refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", got.Token.RefreshToken, "1//refresh-token")
	}
	if got.Config.ClientID != "file-client-id.apps.googleusercontent.com" {
		t.Errorf("ClientID = %q, want file-client-id", got.Config.ClientID)
	}
	if got.Config.ClientSecret != "file-client-secret" {
		t.Errorf("ClientSecret = %q, want file-client-secret", got.Config.ClientSecret)
	}
	if got.Config.Endpoint.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("TokenURL = %q", got.Config.Endpoint.TokenURL)
	}
	if len(got.Config.Scopes) != 2 {
		t.Errorf("Scopes len = %d, want 2", len(got.Config.Scopes))
	}

	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if !got.Token.Expiry.Equal(want) {
		t.Errorf("Expiry = %v, want %v", got.Token.Expiry, want)
	}
}

func TestGetCredential_NaiveExpiry(t *testing.T) {
	dir := t.TempDir()
	email := "naive@example.com"
	cred := credentialJSON{
		Token:  "tok",
		Expiry: "2026-03-01T12:00:00",
	}
	writeCred(t, dir, email, cred)

	store := &LocalDirectoryCredentialStore{Dir: dir}
	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	want := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if !got.Token.Expiry.Equal(want) {
		t.Errorf("Expiry = %v, want %v", got.Token.Expiry, want)
	}
}

func TestGetCredential_MissingFile(t *testing.T) {
	store := &LocalDirectoryCredentialStore{Dir: t.TempDir()}
	got, err := store.GetCredential("nobody@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing file")
	}
}

func TestGetCredential_NoExpiry(t *testing.T) {
	dir := t.TempDir()
	email := "noexp@example.com"
	cred := credentialJSON{Token: "tok"}
	writeCred(t, dir, email, cred)

	store := &LocalDirectoryCredentialStore{Dir: dir}
	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if !got.Token.Expiry.IsZero() {
		t.Errorf("Expiry should be zero, got %v", got.Token.Expiry)
	}
}

func TestGetCredential_DefaultTokenURI(t *testing.T) {
	dir := t.TempDir()
	email := "default@example.com"
	cred := credentialJSON{Token: "tok"}
	writeCred(t, dir, email, cred)

	store := &LocalDirectoryCredentialStore{Dir: dir}
	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if got.Config.Endpoint.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("TokenURL = %q, want default", got.Config.Endpoint.TokenURL)
	}
}

func TestGetCredential_EnvVarOverrides(t *testing.T) {
	dir := t.TempDir()
	email := "envtest@example.com"
	cred := credentialJSON{
		Token:        "tok",
		ClientID:     "from-file",
		ClientSecret: "from-file",
	}
	writeCred(t, dir, email, cred)

	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "env-client-id")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "env-client-secret")

	store := &LocalDirectoryCredentialStore{Dir: dir}
	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if got.Config.ClientID != "env-client-id" {
		t.Errorf("ClientID = %q, want env-client-id", got.Config.ClientID)
	}
	if got.Config.ClientSecret != "env-client-secret" {
		t.Errorf("ClientSecret = %q, want env-client-secret", got.Config.ClientSecret)
	}
}

func TestStoreCredential_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	email := "roundtrip@example.com"
	store := &LocalDirectoryCredentialStore{Dir: dir}

	tok := oauth2Token("access-tok", "refresh-tok", time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC))
	original := &StoredCredential{
		Token:  &tok,
		Config: newTestConfig("cid", "csecret"),
	}

	if err := store.StoreCredential(email, original); err != nil {
		t.Fatalf("StoreCredential: %v", err)
	}

	// Clear env so file values are used
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "")

	got, err := store.GetCredential(email)
	if err != nil {
		t.Fatalf("GetCredential after store: %v", err)
	}
	if got.Token.AccessToken != "access-tok" {
		t.Errorf("AccessToken = %q", got.Token.AccessToken)
	}
	if got.Token.RefreshToken != "refresh-tok" {
		t.Errorf("RefreshToken = %q", got.Token.RefreshToken)
	}
	if got.Config.ClientID != "cid" {
		t.Errorf("ClientID = %q", got.Config.ClientID)
	}
}

func TestDeleteCredential(t *testing.T) {
	dir := t.TempDir()
	email := "delete@example.com"
	cred := credentialJSON{Token: "tok"}
	writeCred(t, dir, email, cred)

	store := &LocalDirectoryCredentialStore{Dir: dir}

	// Verify it exists
	got, err := store.GetCredential(email)
	if err != nil || got == nil {
		t.Fatal("credential should exist before delete")
	}

	if err := store.DeleteCredential(email); err != nil {
		t.Fatalf("DeleteCredential: %v", err)
	}

	got, err = store.GetCredential(email)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if got != nil {
		t.Error("credential should be nil after delete")
	}
}

func TestDeleteCredential_Missing(t *testing.T) {
	store := &LocalDirectoryCredentialStore{Dir: t.TempDir()}
	if err := store.DeleteCredential("nobody@example.com"); err != nil {
		t.Fatalf("deleting nonexistent should not error: %v", err)
	}
}

func TestListUsers(t *testing.T) {
	dir := t.TempDir()
	for _, email := range []string{"alice@example.com", "bob@example.com"} {
		writeCred(t, dir, email, credentialJSON{Token: "tok"})
	}
	// Non-json file should be ignored
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o600)

	store := &LocalDirectoryCredentialStore{Dir: dir}
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("ListUsers returned %d users, want 2", len(users))
	}
	found := map[string]bool{}
	for _, u := range users {
		found[u] = true
	}
	if !found["alice@example.com"] || !found["bob@example.com"] {
		t.Errorf("users = %v, expected alice and bob", users)
	}
}

func TestListUsers_EmptyDir(t *testing.T) {
	store := &LocalDirectoryCredentialStore{Dir: t.TempDir()}
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}

func TestListUsers_MissingDir(t *testing.T) {
	store := &LocalDirectoryCredentialStore{Dir: filepath.Join(t.TempDir(), "nonexistent")}
	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers on missing dir should not error: %v", err)
	}
	if users != nil {
		t.Errorf("expected nil, got %v", users)
	}
}

func TestNewCredentialStore(t *testing.T) {
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", "/custom/workspace")
	t.Setenv("GOOGLE_MCP_CREDENTIALS_DIR", "/custom/google")
	store := NewCredentialStore()
	if store == nil || store.Dir != "/custom/workspace" {
		t.Fatalf("unexpected store: %+v", store)
	}
}

func TestResolveCredentialDir_EnvPriority(t *testing.T) {
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", "/custom/workspace")
	t.Setenv("GOOGLE_MCP_CREDENTIALS_DIR", "/custom/google")
	got := resolveCredentialDir()
	if got != "/custom/workspace" {
		t.Errorf("resolveCredentialDir = %q, want /custom/workspace (WORKSPACE env wins)", got)
	}
}

func TestResolveCredentialDir_FallbackToGoogle(t *testing.T) {
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", "")
	t.Setenv("GOOGLE_MCP_CREDENTIALS_DIR", "/custom/google")
	got := resolveCredentialDir()
	if got != "/custom/google" {
		t.Errorf("resolveCredentialDir = %q, want /custom/google", got)
	}
}

func TestResolveCredentialDir_Default(t *testing.T) {
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", "")
	t.Setenv("GOOGLE_MCP_CREDENTIALS_DIR", "")
	got := resolveCredentialDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".google_workspace_mcp", "credentials")
	if got != want {
		t.Errorf("resolveCredentialDir = %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		input string
		want  string
	}{
		{"~/creds", filepath.Join(home, "creds")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}
	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// helpers

func writeCred(t *testing.T, dir, email string, cj credentialJSON) {
	t.Helper()
	data, err := json.Marshal(cj)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, email+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func oauth2Token(access, refresh string, expiry time.Time) oauth2.Token {
	return oauth2.Token{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		Expiry:       expiry,
	}
}

func newTestConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"},
	}
}
