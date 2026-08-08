package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	pendingFileName = "oauth_pending.json"
	pendingTTL      = 10 * time.Minute
)

// PendingSession is the on-disk state written by "auth url" and consumed by
// "auth exchange". It holds the PKCE verifier and state so that the two
// commands can run in separate processes.
type PendingSession struct {
	Verifier    string    `json:"verifier"`
	State       string    `json:"state"`
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Expired reports whether the pending session has passed its TTL.
func (p *PendingSession) Expired() bool {
	return time.Now().After(p.ExpiresAt)
}

func pendingPath(credDir string) string {
	return filepath.Join(credDir, pendingFileName)
}

// SavePendingSession atomically writes a PendingSession to the credential
// directory with mode 0600.
func SavePendingSession(credDir string, ps *PendingSession) error {
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		return fmt.Errorf("creating credential directory: %w", err)
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling pending session: %w", err)
	}
	return os.WriteFile(pendingPath(credDir), data, 0o600)
}

// LoadPendingSession reads a PendingSession from disk. Returns a clear error
// if the file is missing or the session has expired (and removes the stale
// file in the latter case).
func LoadPendingSession(credDir string) (*PendingSession, error) {
	data, err := os.ReadFile(pendingPath(credDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("no pending auth session found; run 'google-mcp auth url' first")
		}
		return nil, fmt.Errorf("reading pending session: %w", err)
	}

	var ps PendingSession
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parsing pending session: %w", err)
	}

	if ps.Expired() {
		_ = DeletePendingSession(credDir)
		return nil, errors.New("pending auth session expired; run 'google-mcp auth url' again")
	}

	return &ps, nil
}

// DeletePendingSession removes the pending file. It is a no-op if the file
// does not exist.
func DeletePendingSession(credDir string) error {
	err := os.Remove(pendingPath(credDir))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing pending session: %w", err)
	}
	return nil
}
