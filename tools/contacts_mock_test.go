package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// contactsTestServer creates an MCP server with specific Contacts tools registered,
// backed by the given fake HTTP server.
func contactsTestServer(t *testing.T, registerFuncs []func(s *mcpserver.MCPServer, getClient httpClientFunc), getClient httpClientFunc) *mcpserver.MCPServer {
	t.Helper()
	t.Setenv("USER_GOOGLE_EMAIL", "test@example.com")
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())
	s := mcpserver.NewMCPServer("test", "0.0.0")
	for _, reg := range registerFuncs {
		reg(s, getClient)
	}
	return s
}

// --- contacts_list ---

func TestContactsMockListContacts(t *testing.T) {
	t.Run("success_with_contacts", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/people/me/connections": map[string]any{
				"connections": []map[string]any{
					{
						"resourceName": "people/c123",
						"names":        []map[string]any{{"displayName": "Alice Smith"}},
						"emailAddresses": []map[string]any{
							{"value": "alice@example.com"},
						},
						"phoneNumbers": []map[string]any{
							{"value": "+1-555-0100"},
						},
					},
					{
						"resourceName": "people/c456",
						"names":        []map[string]any{{"displayName": "Bob Jones"}},
						"emailAddresses": []map[string]any{
							{"value": "bob@example.com"},
						},
					},
				},
				"totalPeople": 2,
			},
		})
		s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListContacts}, testClientFunc(ts))
		text, isError := callTool(t, s, "contacts_list", map[string]any{
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Contacts for test@example.com") {
			t.Errorf("expected contacts header, got:\n%s", text)
		}
		if !strings.Contains(text, "Alice Smith") {
			t.Errorf("expected 'Alice Smith' in output")
		}
		if !strings.Contains(text, "Bob Jones") {
			t.Errorf("expected 'Bob Jones' in output")
		}
		if !strings.Contains(text, "alice@example.com") {
			t.Errorf("expected email in output")
		}
	})

	t.Run("success_no_contacts", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/people/me/connections": map[string]any{
				"connections": []map[string]any{},
				"totalPeople": 0,
			},
		})
		s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListContacts}, testClientFunc(ts))
		text, isError := callTool(t, s, "contacts_list", map[string]any{
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "No contacts found") {
			t.Errorf("expected 'No contacts found', got:\n%s", text)
		}
	})
}

// --- contacts_create ---

func TestContactsMockCreateContact(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/people:createContact": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"resourceName": "people/c789",
					"names": [{"displayName": "Carol White"}],
					"emailAddresses": [{"value": "carol@example.com"}]
				}`)
			},
		})
		s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerCreateContact}, testClientFunc(ts))
		text, isError := callTool(t, s, "contacts_create", map[string]any{
			"given_name":        "Carol",
			"family_name":       "White",
			"email":             "carol@example.com",
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Contact Created") {
			t.Errorf("expected 'Contact Created', got:\n%s", text)
		}
		if !strings.Contains(text, "Carol") {
			t.Errorf("expected name in output")
		}
	})
}

// --- contacts_list_groups ---

func TestContactsMockListContactGroups(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/contactGroups": map[string]any{
				"contactGroups": []map[string]any{
					{"resourceName": "contactGroups/myContacts", "name": "My Contacts", "groupType": "SYSTEM_CONTACT_GROUP", "memberCount": 25},
					{"resourceName": "contactGroups/friends", "name": "Friends", "groupType": "USER_CONTACT_GROUP", "memberCount": 10},
				},
			},
		})
		s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListContactGroups}, testClientFunc(ts))
		text, isError := callTool(t, s, "contacts_list_groups", map[string]any{
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "My Contacts") {
			t.Errorf("expected 'My Contacts' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Friends") {
			t.Errorf("expected 'Friends' in output")
		}
	})
}

func TestContactsMockCreateContactGroup(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/contactGroups": map[string]any{
			"resourceName": "contactGroups/project-team",
			"name":         "Project Team",
			"groupType":    "USER_CONTACT_GROUP",
		},
	})
	s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerCreateContactGroup}, testClientFunc(ts))
	text, isError := callTool(t, s, "contacts_create_group", map[string]any{
		"name":              "Project Team",
		"user_google_email": "test@example.com",
	})
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Contact Group Created") ||
		!strings.Contains(text, "Project Team") ||
		!strings.Contains(text, "project-team") {
		t.Errorf("expected group creation result, got:\n%s", text)
	}
}

// --- API error responses ---

func TestContactsMockAPIError(t *testing.T) {
	t.Run("list_contacts_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v1/people/me/connections": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		s := contactsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListContacts}, testClientFunc(ts))
		text, isError := callTool(t, s, "contacts_list", map[string]any{
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error, got success: %s", text)
		}
		if !strings.Contains(text, "listing contacts") {
			t.Errorf("expected listing contacts error, got:\n%s", text)
		}
	})
}
