package tools

import (
	"context"
	"net/http"

	"github.com/shotah/google-mcp/internal/google"
)

// httpClientFunc returns an authenticated *http.Client for the given user
// email. It abstracts the credential lookup so that production code can use
// the real ClientCache while tests inject fake HTTP clients.
type httpClientFunc func(ctx context.Context, email string) (*http.Client, error)

// clientFuncFromCache wraps a *google.ClientCache as an httpClientFunc.
func clientFuncFromCache(cc *google.ClientCache) httpClientFunc {
	return func(ctx context.Context, email string) (*http.Client, error) {
		return cc.GetAuthenticatedClient(ctx, email)
	}
}
