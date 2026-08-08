package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	defaultRedirectURI = "https://shotah.github.io/ai-gantry/oauth-catch/"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
)

// tokenForEmail is the subset of *oauth2.Token needed by the email-fetch
// function. Using an alias keeps the test-seam minimal.
type tokenForEmail = oauth2.Token

// fetchUserEmailFn is the function used to resolve the authenticated user's
// email after a token exchange. Tests replace it to avoid live HTTP calls.
var fetchUserEmailFn func(tok *tokenForEmail) (string, error) = fetchUserEmail

// ManualRedirectURI returns the redirect URI for the non-interactive
// (url / exchange) flow: GOOGLE_OAUTH_REDIRECT_URI env var, or the
// default catch page.
func ManualRedirectURI() string {
	if uri := os.Getenv("GOOGLE_OAUTH_REDIRECT_URI"); uri != "" {
		return uri
	}
	return defaultRedirectURI
}

// GenerateAuthURL builds a Google OAuth authorize URL with PKCE S256,
// persists a PendingSession to credDir, and returns the URL. It does
// NOT open a browser or start a local server.
func GenerateAuthURL(credDir string) (string, error) {
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", errors.New(
			"OAuth client credentials not found. Please set GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET environment variables",
		)
	}

	verifier := oauth2.GenerateVerifier()

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("generating state token: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	redirectURI := ManualRedirectURI()

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: googleTokenURL,
		},
		Scopes:      DefaultScopes,
		RedirectURL: redirectURI,
	}

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier),
	)

	now := time.Now()
	ps := &PendingSession{
		Verifier:    verifier,
		State:       state,
		RedirectURI: redirectURI,
		CreatedAt:   now,
		ExpiresAt:   now.Add(pendingTTL),
	}
	if err := SavePendingSession(credDir, ps); err != nil {
		return "", err
	}

	return authURL, nil
}

// ExchangeAuthCode loads a pending session, exchanges the authorization code
// for tokens using PKCE, stores credentials, and cleans up the pending file.
// It returns the authenticated email and credential file path.
func ExchangeAuthCode(
	ctx context.Context,
	credDir string,
	code string,
	store *LocalDirectoryCredentialStore,
) (string, string, error) {
	return ExchangeAuthCodeWithTokenURL(ctx, credDir, code, store, googleTokenURL)
}

// ExchangeAuthCodeWithTokenURL is the same as ExchangeAuthCode but allows
// callers (and tests) to override the token endpoint URL.
func ExchangeAuthCodeWithTokenURL(
	ctx context.Context,
	credDir string,
	code string,
	store *LocalDirectoryCredentialStore,
	tokenURL string,
) (email string, path string, err error) {
	ps, err := LoadPendingSession(credDir)
	if err != nil {
		return "", "", err
	}

	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return "", "", errors.New(
			"OAuth client credentials not found. Please set GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET environment variables",
		)
	}

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: tokenURL,
		},
		Scopes:      DefaultScopes,
		RedirectURL: ps.RedirectURI,
	}

	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(ps.Verifier))
	if err != nil {
		return "", "", fmt.Errorf("OAuth token exchange: %w", err)
	}

	email, err = fetchUserEmailFn(tok)
	if err != nil {
		// Same fallback as interactive auth: operator-supplied email.
		if fallback := strings.TrimSpace(os.Getenv("USER_GOOGLE_EMAIL")); fallback != "" {
			email = fallback
		} else {
			return "", "", fmt.Errorf("fetch user email: %w", err)
		}
	}

	cred := &StoredCredential{
		Token:  tok,
		Config: cfg,
	}
	if err := store.StoreCredential(email, cred); err != nil {
		return "", "", fmt.Errorf("store credentials for %s: %w", email, err)
	}

	_ = DeletePendingSession(credDir)

	return email, store.CredentialPath(email), nil
}
