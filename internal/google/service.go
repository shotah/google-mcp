// Package google provides authenticated Google API client construction.
package google

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	_ "google.golang.org/api/calendar/v3"
	_ "google.golang.org/api/chat/v1"
	_ "google.golang.org/api/customsearch/v1"
	_ "google.golang.org/api/docs/v1"
	_ "google.golang.org/api/drive/v3"
	_ "google.golang.org/api/forms/v1"
	_ "google.golang.org/api/gmail/v1"
	_ "google.golang.org/api/people/v1"
	_ "google.golang.org/api/script/v1"
	_ "google.golang.org/api/sheets/v4"
	_ "google.golang.org/api/slides/v1"
	_ "google.golang.org/api/tasks/v1"

	"github.com/shotah/google-mcp/auth"
)

// cachedClient holds a cached HTTP client along with the credential store
// reference needed to refresh tokens.
type cachedClient struct {
	client *http.Client
	cred   *auth.StoredCredential
}

// ClientCache caches authenticated HTTP clients by user email.
// It is safe for concurrent use.
type ClientCache struct {
	mu      sync.RWMutex
	clients map[string]*cachedClient
	store   *auth.LocalDirectoryCredentialStore
}

// NewClientCache creates a new ClientCache backed by the given credential store.
func NewClientCache(store *auth.LocalDirectoryCredentialStore) *ClientCache {
	return &ClientCache{
		clients: make(map[string]*cachedClient),
		store:   store,
	}
}

// GetAuthenticatedClient returns an *http.Client with a valid OAuth token
// for the given user email. The underlying oauth2.Transport automatically
// refreshes expired access tokens using the stored refresh token.
//
// Clients are cached by email so repeated calls for the same user reuse
// the same transport (and its token source).
func (c *ClientCache) GetAuthenticatedClient(ctx context.Context, userEmail string) (*http.Client, error) {
	// Fast path: check read lock for existing cached client.
	c.mu.RLock()
	if cached, ok := c.clients[userEmail]; ok {
		c.mu.RUnlock()
		return cached.client, nil
	}
	c.mu.RUnlock()

	// Slow path: load credentials and build client.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if cached, ok := c.clients[userEmail]; ok {
		return cached.client, nil
	}

	cred, err := c.store.GetCredential(userEmail)
	if err != nil {
		return nil, fmt.Errorf("loading credentials for %s: %w (run: google-mcp auth)", userEmail, err)
	}
	if cred == nil {
		return nil, fmt.Errorf("no credentials found for %s; run: google-mcp auth", userEmail)
	}

	// oauth2.Config.Client returns an *http.Client whose Transport
	// automatically refreshes expired access tokens using the refresh token.
	// Agents should not call auth_start for routine use — CLI setup + this refresh is enough.
	client := cred.Config.Client(ctx, cred.Token)

	c.clients[userEmail] = &cachedClient{
		client: client,
		cred:   cred,
	}

	return client, nil
}

// Invalidate removes a cached client for the given user email,
// forcing the next call to reload credentials from disk.
func (c *ClientCache) Invalidate(userEmail string) {
	c.mu.Lock()
	delete(c.clients, userEmail)
	c.mu.Unlock()
}

var (
	defaultCache     *ClientCache
	defaultCacheOnce sync.Once
)

// DefaultClientCache returns a process-wide singleton ClientCache
// backed by the default credential store.
func DefaultClientCache() *ClientCache {
	defaultCacheOnce.Do(func() {
		defaultCache = NewClientCache(auth.NewCredentialStore())
	})
	return defaultCache
}
