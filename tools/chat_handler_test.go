package tools

import (
	"strings"
	"testing"
)

// --- chat_list_spaces ---
// chat_list_spaces has no strictly required params (email resolved via env),
// so the first error path is auth failure.

func TestChatHandlerListSpacesAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_list_spaces", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- chat_list_messages ---

func TestChatHandlerGetMessagesMissingSpaceID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_list_messages", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "space_id") {
		t.Errorf("expected error mentioning 'space_id', got %q", text)
	}
	if !strings.Contains(text, "Next:") || !strings.Contains(text, "chat_list_spaces") {
		t.Errorf("expected Next: teach-in via chat_list_spaces, got %q", text)
	}
}

func TestChatHandlerGetMessagesAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_list_messages", map[string]any{
		"space_id": "spaces/abc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- chat_send_message ---

func TestChatHandlerSendMessageMissingSpaceID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_send_message", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "space_id") {
		t.Errorf("expected error mentioning 'space_id', got %q", text)
	}
	if !strings.Contains(text, "Next:") || !strings.Contains(text, "chat_list_spaces") {
		t.Errorf("expected Next: teach-in via chat_list_spaces, got %q", text)
	}
}

func TestChatHandlerSendMessageMissingMessageText(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_send_message", map[string]any{
		"space_id": "spaces/abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_text") {
		t.Errorf("expected error mentioning 'message_text', got %q", text)
	}
}

// --- chat_search_messages ---

func TestChatHandlerSearchMessagesMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_search_messages", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error mentioning 'query', got %q", text)
	}
}

func TestChatHandlerSearchMessagesAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "chat_search_messages", map[string]any{
		"query": "test message",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}
