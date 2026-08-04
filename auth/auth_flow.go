package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// CallbackResult holds the result from the OAuth callback.
type CallbackResult struct {
	Code  string
	State string
	Error string
}

// authSession is an in-progress localhost OAuth callback flow.
type authSession struct {
	oauthConfig *oauth2.Config
	authURL     string
	resultCh    <-chan CallbackResult
	srv         *http.Server
	userEmail   string
	oauthCtx    context.Context
}

// StartAuthFlow initiates the Google OAuth flow for the given user.
// It starts a local HTTP server to handle the callback, generates the
// authorization URL, and returns a message for the user to follow.
//
// The callback server runs in the background and stores credentials
// when the user completes the flow.
//
// Prefer RunInteractiveAuth (google-mcp auth) for first-time setup;
// this remains as a rare MCP re-auth escape hatch.
func StartAuthFlow(
	ctx context.Context,
	serviceName string,
	userEmail string,
	store *LocalDirectoryCredentialStore,
	onCredentialStored ...func(email string),
) (string, error) {
	sess, err := beginAuthSession(ctx, userEmail)
	if err != nil {
		return "", err
	}

	// Handle the callback asynchronously
	go func() {
		email, err := sess.waitAndStore(store, onCredentialStored...)
		if err != nil {
			log.Printf("OAuth flow error: %v", err)
			return
		}
		log.Printf("Successfully stored credentials for %s", email)
	}()

	return authStartMessage(serviceName, userEmail, sess.authURL), nil
}

// RunInteractiveAuth runs a blocking OAuth login for humans (CLI).
// It prints the authorization URL, optionally opens a browser, waits for the
// callback, and stores credentials under the credential directory.
//
// openURL may be nil to skip opening a browser (caller still gets the URL on stderr).
func RunInteractiveAuth(
	ctx context.Context,
	userEmail string,
	store *LocalDirectoryCredentialStore,
	openURL func(string) error,
) (email string, path string, err error) {
	sess, err := beginAuthSession(ctx, userEmail)
	if err != nil {
		return "", "", err
	}

	fmt.Fprintln(os.Stderr, "Authorize google-mcp in your browser.")
	fmt.Fprintf(os.Stderr, "If the browser does not open, visit:\n\n%s\n\n", sess.authURL)
	if openURL != nil {
		if err := openURL(sess.authURL); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
		}
	}

	email, err = sess.waitAndStore(store)
	if err != nil {
		return "", "", err
	}
	return email, store.CredentialPath(email), nil
}

// OpenBrowser opens url in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func beginAuthSession(ctx context.Context, userEmail string) (*authSession, error) {
	clientID := os.Getenv("GOOGLE_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("OAuth client credentials not found. Please set GOOGLE_OAUTH_CLIENT_ID and " +
			"GOOGLE_OAUTH_CLIENT_SECRET environment variables",
		)
	}

	// Generate CSRF state token
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generating state token: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Callback must be reachable from the host browser. Default port 4100 + bind
	// 0.0.0.0 so Docker Desktop can publish -p 127.0.0.1:4100:4100 (localhost-only
	// bind inside a container is invisible to published ports). Redirect URI stays
	// http://localhost:<port>/… for the Google OAuth client.
	resultCh := make(chan CallbackResult, 1)
	srv, port, err := startCallbackServer(state, resultCh)
	if err != nil {
		return nil, fmt.Errorf("starting OAuth callback server: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/oauth2callback", port)

	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
		Scopes:      DefaultScopes,
		RedirectURL: redirectURI,
	}

	authURL := oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	// OAuth callback outlives a short tool-call context; keep values but not cancellation.
	oauthCtx := context.WithoutCancel(ctx)

	return &authSession{
		oauthConfig: oauthConfig,
		authURL:     authURL,
		resultCh:    resultCh,
		srv:         srv,
		userEmail:   userEmail,
		oauthCtx:    oauthCtx,
	}, nil
}

func (s *authSession) waitAndStore(
	store *LocalDirectoryCredentialStore,
	onCredentialStored ...func(email string),
) (string, error) {
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(s.oauthCtx, 3*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("OAuth callback server shutdown error: %v", err)
		}
	}()

	var result CallbackResult
	select {
	case result = <-s.resultCh:
	case <-time.After(5 * time.Minute):
		return "", errors.New("OAuth callback timed out after 5 minutes")
	case <-s.oauthCtx.Done():
		return "", s.oauthCtx.Err()
	}

	if result.Error != "" {
		return "", errors.New(result.Error)
	}

	tok, err := s.oauthConfig.Exchange(s.oauthCtx, result.Code)
	if err != nil {
		return "", fmt.Errorf("OAuth token exchange: %w", err)
	}

	email, err := fetchUserEmail(tok)
	if err != nil {
		if s.userEmail != "" {
			email = s.userEmail
		} else {
			return "", fmt.Errorf("fetch user email: %w", err)
		}
	}

	cred := &StoredCredential{
		Token:  tok,
		Config: s.oauthConfig,
	}
	if err := store.StoreCredential(email, cred); err != nil {
		return "", fmt.Errorf("store credentials for %s: %w", email, err)
	}

	for _, fn := range onCredentialStored {
		fn(email)
	}
	return email, nil
}

func authStartMessage(serviceName, userEmail, authURL string) string {
	initialEmailProvided := userEmail != "" &&
		strings.TrimSpace(userEmail) != "" &&
		!strings.EqualFold(userEmail, "default")

	userDisplayName := serviceName
	if initialEmailProvided {
		userDisplayName = fmt.Sprintf("%s for '%s'", serviceName, userEmail)
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "**ACTION REQUIRED: Google Authentication Needed for %s**\n\n", userDisplayName)
	msg.WriteString("Prefer first-time setup via CLI: `google-mcp auth` (or `google-mcp login`).\n\n")
	fmt.Fprintf(&msg, "To proceed, the user must authorize this application for %s access using all required permissions.\n", serviceName)
	msg.WriteString("**LLM, please present this exact authorization URL to the user as a clickable hyperlink:**\n")
	fmt.Fprintf(&msg, "Authorization URL: %s\n", authURL)
	fmt.Fprintf(&msg, "Markdown for hyperlink: [Click here to authorize %s access](%s)\n\n", serviceName, authURL)
	msg.WriteString("**LLM, after presenting the link, instruct the user as follows:**\n")
	msg.WriteString("1. Click the link and complete the authorization in their browser.\n")

	if !initialEmailProvided {
		msg.WriteString("2. After successful authorization, the browser page will display the authenticated email address.\n")
		msg.WriteString("   **LLM: Instruct the user to provide you with this email address.**\n")
		msg.WriteString("3. Once you have the email, **retry their original command, ensuring you include this `user_google_email`.**\n")
	} else {
		msg.WriteString("2. After successful authorization, **retry their original command**.\n")
	}

	fmt.Fprintf(&msg, "\nThe application will use the new credentials. If '%s' was provided, it must match the authenticated account.", userEmail)

	return msg.String()
}

const defaultOAuthCallbackPort = 4100

// callbackListenPort returns the TCP port for the OAuth callback server.
// GOOGLE_OAUTH_PORT or GOOGLE_AUTH_PORT overrides the default (4100).
func callbackListenPort() int {
	for _, key := range []string{"GOOGLE_OAUTH_PORT", "GOOGLE_AUTH_PORT"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			p, err := strconv.Atoi(raw)
			if err == nil && p > 0 && p < 65536 {
				return p
			}
		}
	}
	return defaultOAuthCallbackPort
}

// callbackBindHost is the interface bind for the callback server.
// Default 0.0.0.0 so Docker -p HOST:CONTAINER can reach it; override with
// GOOGLE_OAUTH_BIND=127.0.0.1 for host-only listen.
func callbackBindHost() string {
	if h := strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_BIND")); h != "" {
		return h
	}
	return "0.0.0.0"
}

// startCallbackServer starts a minimal HTTP server that handles the OAuth callback.
// It returns the server and the port actually bound (for the redirect URI).
func startCallbackServer(expectedState string, resultCh chan<- CallbackResult) (*http.Server, int, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		errParam := query.Get("error")
		if errParam != "" {
			resultCh <- CallbackResult{Error: "Google returned error: " + errParam}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>Error: %s</p><p>You can close this window.</p></body></html>", html.EscapeString(errParam))
			return
		}

		code := query.Get("code")
		state := query.Get("state")

		if code == "" {
			resultCh <- CallbackResult{Error: "no authorization code received"}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h1>Authentication Failed</h1><p>No authorization code received.</p><p>You can close this window.</p></body></html>")
			return
		}

		if state != expectedState {
			resultCh <- CallbackResult{Error: "state mismatch — possible CSRF attack"}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h1>Authentication Failed</h1><p>State mismatch.</p><p>You can close this window.</p></body></html>")
			return
		}

		resultCh <- CallbackResult{Code: code, State: state}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h1>Authentication Successful!</h1><p>You can close this window and return to the application.</p></body></html>")
	})

	bindHost := callbackBindHost()
	port := callbackListenPort()
	addr := net.JoinHostPort(bindHost, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Fixed port busy — fall back to ephemeral on the same bind host.
		addr = net.JoinHostPort(bindHost, "0")
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return nil, 0, err
		}
		fmt.Fprintf(os.Stderr, "oauth: port %d busy; using ephemeral %s (Docker -p may not work)\n", port, ln.Addr())
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, 0, errors.New("unexpected listener address type")
	}
	boundPort := tcpAddr.Port

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("OAuth callback server error: %v", err)
		}
	}()

	return srv, boundPort, nil
}

// fetchUserEmail fetches the authenticated user's email address using the OAuth2 userinfo endpoint.
func fetchUserEmail(tok *oauth2.Token) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, body)
	}

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decoding user info: %w", err)
	}
	if info.Email == "" {
		return "", errors.New("no email in userinfo response")
	}
	return info.Email, nil
}
