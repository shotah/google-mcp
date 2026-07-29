package tools

import (
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
