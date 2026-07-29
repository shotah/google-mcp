package tools

import (
	"strings"
	"testing"
)

// Search tools use GOOGLE_PSE_API_KEY and GOOGLE_PSE_ENGINE_ID env vars
// rather than Google OAuth credentials. The error path is:
// resolveEmail → RequireString params → newCustomSearchService (env var check).

// --- search_query ---

func TestSearchHandlerSearchCustomMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "search_query", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "q") {
		t.Errorf("expected error mentioning 'q', got %q", text)
	}
}

func TestSearchHandlerSearchCustomMissingAPIKey(t *testing.T) {
	s := newToolTestServer(t)
	// Ensure PSE env vars are not set
	t.Setenv("GOOGLE_PSE_API_KEY", "")
	t.Setenv("GOOGLE_PSE_ENGINE_ID", "")
	text, isError := callTool(t, s, "search_query", map[string]any{
		"q": "test query",
	})
	if !isError {
		t.Fatal("expected isError=true for missing API key")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "google_pse_api_key") {
		t.Errorf("expected error mentioning 'GOOGLE_PSE_API_KEY', got %q", text)
	}
}

// --- search_get_engine_info ---
// No required params beyond email; first real error is missing API key.

func TestSearchHandlerGetSearchEngineInfoMissingAPIKey(t *testing.T) {
	s := newToolTestServer(t)
	t.Setenv("GOOGLE_PSE_API_KEY", "")
	t.Setenv("GOOGLE_PSE_ENGINE_ID", "")
	text, isError := callTool(t, s, "search_get_engine_info", nil)
	if !isError {
		t.Fatal("expected isError=true for missing API key")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "google_pse_api_key") {
		t.Errorf("expected error mentioning 'GOOGLE_PSE_API_KEY', got %q", text)
	}
}

// --- search_query_siterestrict ---

func TestSearchHandlerSearchCustomSiterestrictMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "search_query_siterestrict", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "q") {
		t.Errorf("expected error mentioning 'q', got %q", text)
	}
}

func TestSearchHandlerSearchCustomSiterestrictMissingSites(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "search_query_siterestrict", map[string]any{
		"q": "test query",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "sites") {
		t.Errorf("expected error mentioning 'sites', got %q", text)
	}
}

func TestSearchHandlerSearchCustomSiterestrictMissingAPIKey(t *testing.T) {
	s := newToolTestServer(t)
	t.Setenv("GOOGLE_PSE_API_KEY", "")
	t.Setenv("GOOGLE_PSE_ENGINE_ID", "")
	text, isError := callTool(t, s, "search_query_siterestrict", map[string]any{
		"q":     "test query",
		"sites": []any{"example.com"},
	})
	if !isError {
		t.Fatal("expected isError=true for missing API key")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "google_pse_api_key") {
		t.Errorf("expected error mentioning 'GOOGLE_PSE_API_KEY', got %q", text)
	}
}
