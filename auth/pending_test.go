package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveLoadPendingSession_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)
	ps := &PendingSession{
		Verifier:    "test-verifier-abc",
		State:       "state-xyz",
		RedirectURI: "https://example.com/callback",
		CreatedAt:   now,
		ExpiresAt:   now.Add(pendingTTL),
	}

	if err := SavePendingSession(dir, ps); err != nil {
		t.Fatalf("SavePendingSession: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, pendingFileName))
		if err != nil {
			t.Fatalf("pending file stat: %v", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("pending file permissions %o allow group/other access", info.Mode().Perm())
		}
	}

	got, err := LoadPendingSession(dir)
	if err != nil {
		t.Fatalf("LoadPendingSession: %v", err)
	}
	if got.Verifier != ps.Verifier {
		t.Errorf("Verifier = %q, want %q", got.Verifier, ps.Verifier)
	}
	if got.State != ps.State {
		t.Errorf("State = %q, want %q", got.State, ps.State)
	}
	if got.RedirectURI != ps.RedirectURI {
		t.Errorf("RedirectURI = %q, want %q", got.RedirectURI, ps.RedirectURI)
	}
}

func TestLoadPendingSession_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPendingSession(dir)
	if err == nil {
		t.Fatal("expected error for missing pending file")
	}
	if got := err.Error(); got != "no pending auth session found; run 'google-mcp auth url' first" {
		t.Errorf("unexpected error: %q", got)
	}
}

func TestLoadPendingSession_Expired(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().Add(-20 * time.Minute)
	ps := &PendingSession{
		Verifier:    "old-verifier",
		State:       "old-state",
		RedirectURI: "https://example.com/callback",
		CreatedAt:   past,
		ExpiresAt:   past.Add(pendingTTL),
	}
	if err := SavePendingSession(dir, ps); err != nil {
		t.Fatalf("SavePendingSession: %v", err)
	}

	_, err := LoadPendingSession(dir)
	if err == nil {
		t.Fatal("expected error for expired pending session")
	}
	if got := err.Error(); got != "pending auth session expired; run 'google-mcp auth url' again" {
		t.Errorf("unexpected error: %q", got)
	}

	if _, statErr := os.Stat(filepath.Join(dir, pendingFileName)); !os.IsNotExist(statErr) {
		t.Error("expired pending file should have been deleted")
	}
}

func TestPendingSession_Expired(t *testing.T) {
	fresh := &PendingSession{ExpiresAt: time.Now().Add(5 * time.Minute)}
	if fresh.Expired() {
		t.Error("session 5 min in the future should not be expired")
	}

	stale := &PendingSession{ExpiresAt: time.Now().Add(-1 * time.Second)}
	if !stale.Expired() {
		t.Error("session 1 second in the past should be expired")
	}
}

func TestDeletePendingSession(t *testing.T) {
	dir := t.TempDir()
	ps := &PendingSession{
		Verifier:  "v",
		State:     "s",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(pendingTTL),
	}
	if err := SavePendingSession(dir, ps); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := DeletePendingSession(dir); err != nil {
		t.Fatalf("DeletePendingSession: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, pendingFileName)); !os.IsNotExist(err) {
		t.Error("pending file should be removed")
	}
}

func TestDeletePendingSession_NoPanic(t *testing.T) {
	if err := DeletePendingSession(t.TempDir()); err != nil {
		t.Fatalf("deleting nonexistent should not error: %v", err)
	}
}

func TestPendingTTL_IsTenMinutes(t *testing.T) {
	if pendingTTL != 10*time.Minute {
		t.Errorf("pendingTTL = %v, want 10m", pendingTTL)
	}
}
