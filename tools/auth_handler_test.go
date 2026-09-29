package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- auth_start ---

func TestAuthHandlerStartGoogleAuthMissingServiceName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "auth_start", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "service_name") {
		t.Errorf("expected error mentioning 'service_name', got %q", text)
	}
}

func TestAuthListAccountsEmpty(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "auth_list_accounts", nil)
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "No Google accounts") {
		t.Fatalf("got %q", text)
	}
}

func TestAuthListAccountsEmailsOnly(t *testing.T) {
	s := newToolTestServer(t)
	dir := os.Getenv("WORKSPACE_MCP_CREDENTIALS_DIR")
	secret := "ya29.should-not-leak"
	body := []byte(`{"token":"` + secret + `","refresh_token":"1//also-secret"}`)
	if err := os.WriteFile(filepath.Join(dir, "Ada@gmail.com.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	text, isError := callTool(t, s, "auth_list_accounts", nil)
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "ada@gmail.com") {
		t.Fatalf("got %q", text)
	}
	if strings.Contains(text, secret) || strings.Contains(text, "refresh_token") {
		t.Fatalf("token material leaked: %q", text)
	}
}
