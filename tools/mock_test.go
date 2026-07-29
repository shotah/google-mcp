package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// urlRewriteTransport is an http.RoundTripper that redirects all HTTP
// requests to a local httptest.Server by rewriting the scheme and host
// portion of each request URL while preserving the path and query.
type urlRewriteTransport struct {
	target *httptest.Server
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to point at the test server.
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.target.URL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}

// testClientFunc returns an httpClientFunc that always succeeds and returns
// an *http.Client whose transport redirects every request to ts.
func testClientFunc(ts *httptest.Server) httpClientFunc {
	return func(ctx context.Context, email string) (*http.Client, error) {
		return &http.Client{Transport: &urlRewriteTransport{target: ts}}, nil
	}
}

// fakeAPIServer creates an httptest.Server that dispatches requests based on
// URL path prefixes. The routes map maps path prefixes to canned responses:
//   - A value of type int is treated as an HTTP status code with an empty body.
//   - A value of type string is served as-is with Content-Type application/json.
//   - Any other value is JSON-marshalled and served as application/json.
//
// If no route matches the request path, the server returns 404.
func fakeAPIServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for prefix, resp := range routes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				switch v := resp.(type) {
				case int:
					w.WriteHeader(v)
				case string:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, v)
				default:
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(v); err != nil {
						t.Errorf("fakeAPIServer: encode response for %s: %v", prefix, err)
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
				return
			}
		}
		t.Logf("fakeAPIServer: unmatched path: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// --- Proof-of-concept test ---

func TestMockSearchGmailMessages(t *testing.T) {
	// Set up a fake API server returning 2 messages.
	ts := fakeAPIServer(t, map[string]any{
		"/gmail/v1/users/me/messages": map[string]any{
			"messages": []map[string]any{
				{"id": "msg001", "threadId": "thread001"},
				{"id": "msg002", "threadId": "thread002"},
			},
			"resultSizeEstimate": 2,
		},
	})

	// Create the handler directly via handleSearchGmailMessages + testClientFunc.
	handler := handleSearchGmailMessages(testClientFunc(ts))

	// Build a CallToolRequest with the required "query" argument.
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "gmail_search_messages",
			Arguments: map[string]any{
				"query":             "in:inbox",
				"user_google_email": "test@example.com",
			},
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		tc := result.Content[0].(mcp.TextContent)
		t.Fatalf("handler returned tool error: %s", tc.Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("handler returned empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	if !strings.Contains(tc.Text, "Found 2 messages") {
		t.Errorf("expected 'Found 2 messages' in output, got:\n%s", tc.Text)
	}
	if !strings.Contains(tc.Text, "msg001") {
		t.Errorf("expected 'msg001' in output, got:\n%s", tc.Text)
	}
	if !strings.Contains(tc.Text, "msg002") {
		t.Errorf("expected 'msg002' in output, got:\n%s", tc.Text)
	}
}
