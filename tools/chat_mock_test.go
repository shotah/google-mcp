package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// --- chat_list_spaces ---

func TestChatMockListSpaces(t *testing.T) {
	t.Run("success_with_spaces", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/spaces": map[string]any{
				"spaces": []map[string]any{
					{"name": "spaces/AAAA", "displayName": "General", "spaceType": "SPACE"},
					{"name": "spaces/BBBB", "displayName": "Engineering", "spaceType": "SPACE"},
				},
			},
		})
		handler := handleListSpaces(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 Chat spaces") {
			t.Errorf("expected 'Found 2 Chat spaces', got:\n%s", text)
		}
		if !strings.Contains(text, "General") {
			t.Errorf("expected 'General' in output")
		}
		if !strings.Contains(text, "Engineering") {
			t.Errorf("expected 'Engineering' in output")
		}
		if !strings.Contains(text, "spaces/AAAA") {
			t.Errorf("expected space ID in output")
		}
	})

	t.Run("success_no_spaces", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/spaces": map[string]any{
				"spaces": []map[string]any{},
			},
		})
		handler := handleListSpaces(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No Chat spaces found") {
			t.Errorf("expected 'No Chat spaces found', got:\n%s", text)
		}
	})
}

// --- chat_send_message ---

func TestChatMockSendMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/spaces/AAAA/messages": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"name":"spaces/AAAA/messages/msg001","text":"Hello team!","createTime":"2026-02-18T10:00:00Z"}`)
			},
			"/v1/spaces/AAAA": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"name":"spaces/AAAA","displayName":"General","spaceType":"SPACE"}`)
			},
		})
		handler := handleSendMessage(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"space_id":          "spaces/AAAA",
			"message_text":      "Hello team!",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Message sent") {
			t.Errorf("expected 'Message sent', got:\n%s", text)
		}
		if !strings.Contains(text, "spaces/AAAA/messages/msg001") {
			t.Errorf("expected message ID in output, got:\n%s", text)
		}
	})
}

// --- chat_list_messages ---

func TestChatMockGetMessages(t *testing.T) {
	t.Run("success_with_messages", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/spaces/AAAA/messages": `{
				"messages": [
					{"name":"spaces/AAAA/messages/msg001","text":"First message","createTime":"2026-02-18T10:00:00Z","sender":{"displayName":"Alice"}},
					{"name":"spaces/AAAA/messages/msg002","text":"Second message","createTime":"2026-02-18T10:01:00Z","sender":{"displayName":"Bob"}}
				]
			}`,
			"/v1/spaces/AAAA": `{"name":"spaces/AAAA","displayName":"General","spaceType":"SPACE"}`,
		})
		handler := handleGetMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"space_id":          "spaces/AAAA",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Messages from 'General'") {
			t.Errorf("expected space name, got:\n%s", text)
		}
		if !strings.Contains(text, "Alice") || !strings.Contains(text, "First message") {
			t.Errorf("expected first message details, got:\n%s", text)
		}
		if !strings.Contains(text, "Bob") || !strings.Contains(text, "msg002") {
			t.Errorf("expected second message details, got:\n%s", text)
		}
	})

	t.Run("success_empty", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/spaces/AAAA/messages": `{"messages":[]}`,
			"/v1/spaces/AAAA":          `{"name":"spaces/AAAA","displayName":"General","spaceType":"SPACE"}`,
		})
		handler := handleGetMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"space_id":          "spaces/AAAA",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No messages found in space 'General'") {
			t.Errorf("expected empty-message result, got:\n%s", text)
		}
	})
}

// --- chat_search_messages ---

func TestChatMockSearchMessages(t *testing.T) {
	t.Run("space_scoped_success", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/spaces/AAAA/messages": `{
				"messages": [
					{"name":"spaces/AAAA/messages/msg001","text":"Project kickoff tomorrow","createTime":"2026-02-18T10:00:00Z","sender":{"displayName":"Alice"}}
				]
			}`,
		})
		handler := handleSearchMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "kickoff",
			"space_id":          "spaces/AAAA",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 1 messages matching 'kickoff' in space 'spaces/AAAA'") {
			t.Errorf("expected scoped search summary, got:\n%s", text)
		}
		if !strings.Contains(text, "Alice") || !strings.Contains(text, "Project kickoff tomorrow") {
			t.Errorf("expected matching message details, got:\n%s", text)
		}
	})

	t.Run("space_scoped_empty", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/spaces/AAAA/messages": `{"messages":[]}`,
		})
		handler := handleSearchMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "missing",
			"space_id":          "spaces/AAAA",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No messages found matching 'missing' in space 'spaces/AAAA'") {
			t.Errorf("expected empty search result, got:\n%s", text)
		}
	})
}

func TestChatMockSearchMessages_AllSpaces(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/v1/spaces": `{
			"spaces": [
				{"name":"spaces/AAAA","displayName":"General"},
				{"name":"spaces/BBBB","displayName":"Engineering"}
			]
		}`,
		"/v1/spaces/AAAA/messages": `{
			"messages": [
				{"name":"spaces/AAAA/messages/msg001","text":"Project kickoff tomorrow","createTime":"2026-02-18T10:00:00Z","sender":{"displayName":"Alice"}}
			]
		}`,
		"/v1/spaces/BBBB/messages": `{
			"messages": [
				{"name":"spaces/BBBB/messages/msg002","text":"Kickoff notes","createTime":"2026-02-18T10:01:00Z","sender":{"displayName":"Bob"}}
			]
		}`,
	})
	handler := handleSearchMessages(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"query":             "kickoff",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Found 2 messages matching 'kickoff' in all accessible spaces") {
		t.Errorf("expected all-spaces search summary, got:\n%s", text)
	}
	if !strings.Contains(text, "General") || !strings.Contains(text, "Engineering") {
		t.Errorf("expected messages from both spaces, got:\n%s", text)
	}
}

// --- API error responses ---

func TestChatMockAPIError(t *testing.T) {
	t.Run("list_spaces_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v1/spaces": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := handleListSpaces(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "listing spaces") {
			t.Errorf("expected listing spaces error, got:\n%s", text)
		}
	})
}
