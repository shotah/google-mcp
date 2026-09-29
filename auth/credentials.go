package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// credentialJSON is the on-disk format stored by the Python server.
// All fields except Token are optional/nullable.
type credentialJSON struct {
	Token        string   `json:"token"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	TokenURI     string   `json:"token_uri,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Expiry       string   `json:"expiry,omitempty"`
}

// StoredCredential holds parsed OAuth2 credentials for a user.
type StoredCredential struct {
	Token  *oauth2.Token
	Config *oauth2.Config
}

// LocalDirectoryCredentialStore reads and writes credential JSON files
// from a local directory (default: ~/.google_workspace_mcp/credentials/).
type LocalDirectoryCredentialStore struct {
	Dir string
}

// NewCredentialStore creates a LocalDirectoryCredentialStore using the
// standard directory resolution order:
//  1. WORKSPACE_MCP_CREDENTIALS_DIR (highest priority)
//  2. GOOGLE_MCP_CREDENTIALS_DIR
//  3. $DATA_DIR/.google_workspace_mcp/credentials (gantry volume)
//  4. ~/.google_workspace_mcp/credentials
func NewCredentialStore() *LocalDirectoryCredentialStore {
	dir := resolveCredentialDir()
	return &LocalDirectoryCredentialStore{Dir: dir}
}

// CanonicalEmail is the credential-file key for an address.
// Comparison is case-insensitive. Plus-addresses stay distinct.
func CanonicalEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GetCredential reads and parses a credential file for the given email.
// Returns nil, nil if the file does not exist.
func (s *LocalDirectoryCredentialStore) GetCredential(email string) (*StoredCredential, error) {
	path, err := s.findCredentialFile(email)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading credential file: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading credential file: %w", err)
	}

	var cj credentialJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil, fmt.Errorf("parsing credential JSON for %s: %w", email, err)
	}

	return parseCred(cj)
}

// StoreCredential writes a credential file for the given email.
// The file name is the canonical (lowercase) address. An older mixed-case
// file for the same address is removed so the account cannot split in two.
func (s *LocalDirectoryCredentialStore) StoreCredential(email string, cred *StoredCredential) error {
	email = CanonicalEmail(email)
	if email == "" {
		return errors.New("email is empty")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("creating credential directory: %w", err)
	}

	cj := credentialJSON{
		Token:        cred.Token.AccessToken,
		RefreshToken: cred.Token.RefreshToken,
		TokenURI:     cred.Config.Endpoint.TokenURL,
		ClientID:     cred.Config.ClientID,
		ClientSecret: cred.Config.ClientSecret,
		Scopes:       cred.Config.Scopes,
	}
	if !cred.Token.Expiry.IsZero() {
		cj.Expiry = cred.Token.Expiry.UTC().Format(time.RFC3339)
	}

	data, err := json.MarshalIndent(cj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling credential JSON: %w", err)
	}

	path := s.credentialPath(email)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	s.removeCaseVariants(email)
	return nil
}

// DeleteCredential removes the credential file for the given email,
// including an older mixed-case filename for the same address.
func (s *LocalDirectoryCredentialStore) DeleteCredential(email string) error {
	email = CanonicalEmail(email)
	if email == "" {
		return nil
	}
	var firstErr error
	for _, path := range s.matchingCredentialFiles(email) {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = fmt.Errorf("deleting credential file: %w", err)
		}
	}
	return firstErr
}

// ListUsers returns the email addresses that have stored credentials.
func (s *LocalDirectoryCredentialStore) ListUsers() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing credential directory: %w", err)
	}

	seen := make(map[string]bool)
	var users []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		email := CanonicalEmail(strings.TrimSuffix(name, ".json"))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		users = append(users, email)
	}
	sort.Strings(users)
	return users, nil
}

// CredentialPath returns the on-disk path for the given email's credential file.
func (s *LocalDirectoryCredentialStore) CredentialPath(email string) string {
	return s.credentialPath(email)
}

func (s *LocalDirectoryCredentialStore) credentialPath(email string) string {
	return filepath.Join(s.Dir, CanonicalEmail(email)+".json")
}

// findCredentialFile returns the canonical path, or a legacy mixed-case
// filename written before addresses were canonicalized.
func (s *LocalDirectoryCredentialStore) findCredentialFile(email string) (string, error) {
	email = CanonicalEmail(email)
	if email == "" {
		return "", os.ErrNotExist
	}
	path := s.credentialPath(email)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	matches := s.matchingCredentialFiles(email)
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}

func (s *LocalDirectoryCredentialStore) matchingCredentialFiles(email string) []string {
	email = CanonicalEmail(email)
	if email == "" {
		return nil
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil
	}
	want := email + ".json"
	var matches []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.EqualFold(name, want) {
			continue
		}
		matches = append(matches, filepath.Join(s.Dir, name))
	}
	return matches
}

func (s *LocalDirectoryCredentialStore) removeCaseVariants(email string) {
	keep := s.credentialPath(email)
	for _, path := range s.matchingCredentialFiles(email) {
		if path != keep {
			_ = os.Remove(path)
		}
	}
}

// resolveCredentialDir determines the credential directory using env vars
// with the standard priority order.
func resolveCredentialDir() string {
	if dir := os.Getenv("WORKSPACE_MCP_CREDENTIALS_DIR"); dir != "" {
		return expandHome(dir)
	}
	if dir := os.Getenv("GOOGLE_MCP_CREDENTIALS_DIR"); dir != "" {
		return expandHome(dir)
	}
	// Gantry bind-mounts DATA_DIR. Distroless HOME is /home/nonroot (overlay),
	// so preferring DATA_DIR is what makes /auth google survive recreate.
	if data := strings.TrimSpace(os.Getenv("DATA_DIR")); data != "" {
		return filepath.Join(expandHome(data), ".google_workspace_mcp", "credentials")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".credentials")
	}
	return filepath.Join(home, ".google_workspace_mcp", "credentials")
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// parseCred converts the on-disk JSON representation into a StoredCredential.
// It applies env var overrides for client_id and client_secret.
func parseCred(cj credentialJSON) (*StoredCredential, error) {
	tok := &oauth2.Token{
		AccessToken:  cj.Token,
		RefreshToken: cj.RefreshToken,
		TokenType:    "Bearer",
	}

	if cj.Expiry != "" {
		expiry, err := parseExpiry(cj.Expiry)
		if err != nil {
			return nil, fmt.Errorf("parsing expiry: %w", err)
		}
		tok.Expiry = expiry
	}

	clientID := cj.ClientID
	clientSecret := cj.ClientSecret
	if envID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID"); envID != "" {
		clientID = envID
	}
	if envSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"); envSecret != "" {
		clientSecret = envSecret
	}

	tokenURI := cj.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: tokenURI,
		},
		Scopes:      cj.Scopes,
		RedirectURL: "http://localhost:4100/code",
	}

	return &StoredCredential{Token: tok, Config: cfg}, nil
}

// parseExpiry attempts to parse an expiry string in multiple ISO 8601 formats.
// The Python server stores timezone-naive UTC datetimes in ISO format.
func parseExpiry(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05.999999999",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized expiry format: %q", s)
}
