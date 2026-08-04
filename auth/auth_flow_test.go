package auth

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestCallbackListenPort(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_PORT", "")
	t.Setenv("GOOGLE_AUTH_PORT", "")
	if got := callbackListenPort(); got != defaultOAuthCallbackPort {
		t.Errorf("default = %d, want %d", got, defaultOAuthCallbackPort)
	}

	t.Setenv("GOOGLE_OAUTH_PORT", "4123")
	if got := callbackListenPort(); got != 4123 {
		t.Errorf("GOOGLE_OAUTH_PORT = %d, want 4123", got)
	}

	t.Setenv("GOOGLE_OAUTH_PORT", "nope")
	t.Setenv("GOOGLE_AUTH_PORT", "4500")
	if got := callbackListenPort(); got != 4500 {
		t.Errorf("GOOGLE_AUTH_PORT fallback = %d, want 4500", got)
	}

	t.Setenv("GOOGLE_OAUTH_PORT", "0")
	t.Setenv("GOOGLE_AUTH_PORT", "")
	if got := callbackListenPort(); got != defaultOAuthCallbackPort {
		t.Errorf("invalid port 0 = %d, want default %d", got, defaultOAuthCallbackPort)
	}
}

func TestCallbackBindHost(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_BIND", "")
	if got := callbackBindHost(); got != "0.0.0.0" {
		t.Errorf("default bind = %q, want 0.0.0.0", got)
	}

	t.Setenv("GOOGLE_OAUTH_BIND", "127.0.0.1")
	if got := callbackBindHost(); got != "127.0.0.1" {
		t.Errorf("bind override = %q, want 127.0.0.1", got)
	}
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestStartCallbackServer_Success(t *testing.T) {
	port := freeLocalPort(t)
	t.Setenv("GOOGLE_OAUTH_BIND", "127.0.0.1")
	t.Setenv("GOOGLE_OAUTH_PORT", strconv.Itoa(port))
	t.Setenv("GOOGLE_AUTH_PORT", "")

	resultCh := make(chan CallbackResult, 1)
	srv, bound, err := startCallbackServer("state123", resultCh)
	if err != nil {
		t.Fatalf("startCallbackServer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if bound != port {
		t.Fatalf("bound port = %d, want %d", bound, port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/oauth2callback?code=abc&state=state123", bound))
	if err != nil {
		t.Fatalf("callback GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Authentication Successful") {
		t.Fatalf("unexpected body: %s", body)
	}

	select {
	case got := <-resultCh:
		if got.Error != "" || got.Code != "abc" || got.State != "state123" {
			t.Fatalf("result = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for callback result")
	}
}

func TestStartCallbackServer_PortBusyFallsBack(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen busy: %v", err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	t.Setenv("GOOGLE_OAUTH_BIND", "127.0.0.1")
	t.Setenv("GOOGLE_OAUTH_PORT", strconv.Itoa(busyPort))
	t.Setenv("GOOGLE_AUTH_PORT", "")

	resultCh := make(chan CallbackResult, 1)
	srv, bound, err := startCallbackServer("s", resultCh)
	if err != nil {
		t.Fatalf("startCallbackServer: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if bound == busyPort {
		t.Fatalf("expected ephemeral fallback away from busy port %d", busyPort)
	}
}
