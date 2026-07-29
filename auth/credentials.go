package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
//  3. ~/.google_workspace_mcp/credentials
func NewCredentialStore() *LocalDirectoryCredentialStore {
	dir := resolveCredentialDir()
	return &LocalDirectoryCredentialStore{Dir: dir}
}

// GetCredential reads and parses a credential file for the given email.
// Returns nil, nil if the file does not exist.
func (s *LocalDirectoryCredentialStore) GetCredential(email string) (*StoredCredential, error) {
	path := s.credentialPath(email)
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
func (s *LocalDirectoryCredentialStore) StoreCredential(email string, cred *StoredCredential) error {
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
	return os.WriteFile(path, data, 0o600)
}

// DeleteCredential removes the credential file for the given email.
func (s *LocalDirectoryCredentialStore) DeleteCredential(email string) error {
	path := s.credentialPath(email)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting credential file: %w", err)
	}
	return nil
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

	var users []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".json") {
			users = append(users, strings.TrimSuffix(name, ".json"))
		}
	}
	return users, nil
}

// CredentialPath returns the on-disk path for the given email's credential file.
func (s *LocalDirectoryCredentialStore) CredentialPath(email string) string {
	return s.credentialPath(email)
}

func (s *LocalDirectoryCredentialStore) credentialPath(email string) string {
	return filepath.Join(s.Dir, email+".json")
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
