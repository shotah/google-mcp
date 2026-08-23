package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// --- resolveEmail ---

func TestResolveEmailParamTakesPriority(t *testing.T) {
	// Set env var to prove it's ignored when param is present.
	t.Setenv("USER_GOOGLE_EMAIL", "env@example.com")

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"user_google_email": "param@example.com",
	}

	got, err := resolveEmail(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "param@example.com" {
		t.Errorf("got %q, want %q", got, "param@example.com")
	}
}

func TestResolveEmailFallsBackToEnvVar(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "env@example.com")
	// Point credential dir to empty temp dir so fallback 3 doesn't trigger.
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	got, err := resolveEmail(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "env@example.com" {
		t.Errorf("got %q, want %q", got, "env@example.com")
	}
}

func TestResolveEmailFallsBackToSingleCredFile(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "")
	credDir := t.TempDir()
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", credDir)

	// Create a single credential file.
	if err := os.WriteFile(filepath.Join(credDir, "single@example.com.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	got, err := resolveEmail(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "single@example.com" {
		t.Errorf("got %q, want %q", got, "single@example.com")
	}
}

func TestResolveEmailErrorWhenNoSource(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "")
	// Point to empty dir — no credential files.
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	_, err := resolveEmail(request)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), EnvUserGoogleEmail+" is required") {
		t.Errorf("error should mention %s is required, got: %v", EnvUserGoogleEmail, err)
	}
}

func TestResolveEmailWhitespaceOnlyParamTreatedAsEmpty(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "env@example.com")

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"user_google_email": "   \t  ",
	}

	got, err := resolveEmail(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to env var since param is whitespace-only.
	if got != "env@example.com" {
		t.Errorf("got %q, want %q", got, "env@example.com")
	}
}

func TestResolveEmailWhitespaceOnlyEnvVarTreatedAsEmpty(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "   \t  ")
	credDir := t.TempDir()
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", credDir)

	// Create a single credential file so fallback 3 works.
	if err := os.WriteFile(filepath.Join(credDir, "cred@example.com.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	got, err := resolveEmail(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should skip env var and fall back to credential file.
	if got != "cred@example.com" {
		t.Errorf("got %q, want %q", got, "cred@example.com")
	}
}
