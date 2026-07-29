//go:build integration

package tools

import (
	"os"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/server"
)

// integrationServer creates an MCP server with all tools registered using real
// credentials. It reads INTEGRATION_TEST_EMAIL from the environment and skips
// the test if not set.
func integrationServer(t *testing.T) (*mcpserver.MCPServer, string) {
	t.Helper()
	email := os.Getenv("INTEGRATION_TEST_EMAIL")
	if email == "" {
		t.Skip("INTEGRATION_TEST_EMAIL not set — skipping integration test")
	}
	cfg := server.Config{}
	s := server.New(cfg)
	RegisterAllTools(s, cfg)
	FilterTools(s, cfg)
	return s, email
}

// --- Gmail integration tests ---

func TestIntegrationSearchGmailMessages(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "gmail_search_messages", map[string]any{
		"query":             "in:inbox",
		"page_size":         1,
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// Should return either results or "No messages found" — both are valid
	if !strings.Contains(text, "messages") && !strings.Contains(text, "Messages") && !strings.Contains(text, "No messages found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

func TestIntegrationListGmailLabels(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "gmail_list_labels", map[string]any{
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	if !strings.Contains(text, "INBOX") {
		t.Errorf("expected INBOX label in output, got:\n%s", text)
	}
}

// --- Drive integration tests ---

func TestIntegrationSearchDriveFiles(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "drive_search_files", map[string]any{
		"query":             "type:document",
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// May return files or "No files found" — both are valid
	if !strings.Contains(text, "files") && !strings.Contains(text, "Files") && !strings.Contains(text, "No files found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

// --- Calendar integration tests ---

func TestIntegrationListCalendars(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "calendar_list_calendars", map[string]any{
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// Every Google account has at least one calendar
	if !strings.Contains(text, "calendar") && !strings.Contains(text, "Calendar") {
		t.Errorf("expected calendar info in output, got:\n%s", text)
	}
}

func TestIntegrationGetEvents(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "calendar_list_events", map[string]any{
		"time_min":          "2026-01-01T00:00:00Z",
		"time_max":          "2026-12-31T23:59:59Z",
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// May return events or "No events found" — both are valid
	if !strings.Contains(text, "event") && !strings.Contains(text, "Event") && !strings.Contains(text, "No events found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

// --- Docs integration tests ---

func TestIntegrationSearchDocs(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "docs_search", map[string]any{
		"query":             "test",
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// May return docs or "No documents found" — both are valid
	if !strings.Contains(text, "document") && !strings.Contains(text, "Document") && !strings.Contains(text, "No documents found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

// --- Sheets integration tests ---

func TestIntegrationListSpreadsheets(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "sheets_list_spreadsheets", map[string]any{
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// May return spreadsheets or "No spreadsheets found" — both are valid
	if !strings.Contains(text, "spreadsheet") && !strings.Contains(text, "Spreadsheet") && !strings.Contains(text, "No spreadsheets found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

// --- Tasks integration tests ---

func TestIntegrationListTaskLists(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "tasks_list_tasklists", map[string]any{
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// Every Google account has a default task list
	if !strings.Contains(text, "Task") && !strings.Contains(text, "task") && !strings.Contains(text, "No task lists found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}

// --- Contacts integration tests ---

func TestIntegrationListContacts(t *testing.T) {
	s, email := integrationServer(t)
	text, isError := callTool(t, s, "contacts_list", map[string]any{
		"user_google_email": email,
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// May return contacts or "No contacts found" — both are valid
	if !strings.Contains(text, "contact") && !strings.Contains(text, "Contact") && !strings.Contains(text, "No contacts found") {
		t.Errorf("unexpected output:\n%s", text)
	}
}
