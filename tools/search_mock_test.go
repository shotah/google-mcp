package tools

import (
	"strings"
	"testing"
)

// Search tools use API keys (GOOGLE_PSE_API_KEY and GOOGLE_PSE_ENGINE_ID)
// instead of OAuth. We can test success paths by setting these env vars and
// pointing the Custom Search API client at a fake server.

// Note: Search tools create their own HTTP clients via option.WithAPIKey,
// so we cannot inject a test HTTP client for success-path testing.
// Instead, we test the env-var error paths which are the primary
// mock scenarios for Search (since it doesn't use OAuth).

// --- missing env var error paths ---

func TestSearchMockMissingAPIKey(t *testing.T) {
	t.Run("missing_api_key", func(t *testing.T) {
		t.Setenv("GOOGLE_PSE_API_KEY", "")
		t.Setenv("GOOGLE_PSE_ENGINE_ID", "test-engine")

		s := newToolTestServer(t)
		text, isError := callTool(t, s, "search_query", map[string]any{
			"q":                 "golang testing",
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error for missing API key, got success: %s", text)
		}
		if !strings.Contains(text, "GOOGLE_PSE_API_KEY") {
			t.Errorf("expected error about GOOGLE_PSE_API_KEY, got:\n%s", text)
		}
	})

	t.Run("missing_engine_id", func(t *testing.T) {
		t.Setenv("GOOGLE_PSE_API_KEY", "test-key")
		t.Setenv("GOOGLE_PSE_ENGINE_ID", "")

		s := newToolTestServer(t)
		text, isError := callTool(t, s, "search_query", map[string]any{
			"q":                 "golang testing",
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error for missing engine ID, got success: %s", text)
		}
		if !strings.Contains(text, "GOOGLE_PSE_ENGINE_ID") {
			t.Errorf("expected error about GOOGLE_PSE_ENGINE_ID, got:\n%s", text)
		}
	})
}

// --- search_get_engine_info ---

func TestSearchMockGetSearchEngineInfo(t *testing.T) {
	t.Run("missing_api_key", func(t *testing.T) {
		t.Setenv("GOOGLE_PSE_API_KEY", "")

		s := newToolTestServer(t)
		text, isError := callTool(t, s, "search_get_engine_info", map[string]any{
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error, got success: %s", text)
		}
		if !strings.Contains(text, "GOOGLE_PSE_API_KEY") {
			t.Errorf("expected API key error, got:\n%s", text)
		}
	})
}
