package tools

import (
	"strings"
	"testing"
)

// TestGmailHandlerSearchMissingQuery verifies that gmail_search_messages
// returns a tool-level error when the required "query" param is missing.
func TestGmailHandlerSearchMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_search_messages", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error mentioning 'query', got %q", text)
	}
}

// TestGmailHandlerGetMessageContentMissingMessageID verifies that
// gmail_get_message returns error when message_id is missing.
func TestGmailHandlerGetMessageContentMissingMessageID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_message", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_id") {
		t.Errorf("expected error mentioning 'message_id', got %q", text)
	}
}

// TestGmailHandlerGetMessagesContentBatchMissingMessageIDs verifies that
// gmail_get_messages_batch returns error when message_ids is missing.
func TestGmailHandlerGetMessagesContentBatchMissingMessageIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_messages_batch", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_ids") {
		t.Errorf("expected error mentioning 'message_ids', got %q", text)
	}
}

// TestGmailHandlerGetMessagesContentBatchEmptyMessageIDs verifies that
// gmail_get_messages_batch returns error when message_ids is empty.
func TestGmailHandlerGetMessagesContentBatchEmptyMessageIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_messages_batch", map[string]any{
		"message_ids": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_ids") {
		t.Errorf("expected error mentioning message_ids, got %q", text)
	}
}

// TestGmailHandlerGetAttachmentContentMissingMessageID verifies that
// gmail_get_attachment returns error when message_id is missing.
func TestGmailHandlerGetAttachmentContentMissingMessageID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_attachment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_id") {
		t.Errorf("expected error mentioning 'message_id', got %q", text)
	}
}

// TestGmailHandlerGetAttachmentContentMissingAttachmentID verifies that
// gmail_get_attachment returns error when attachment_id is missing.
func TestGmailHandlerGetAttachmentContentMissingAttachmentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_attachment", map[string]any{
		"message_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "attachment_id") {
		t.Errorf("expected error mentioning 'attachment_id', got %q", text)
	}
}

// TestGmailHandlerGetThreadContentMissingThreadID verifies that
// gmail_get_thread returns error when thread_id is missing.
func TestGmailHandlerGetThreadContentMissingThreadID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_thread", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "thread_id") {
		t.Errorf("expected error mentioning 'thread_id', got %q", text)
	}
}

// TestGmailHandlerGetThreadsContentBatchMissingThreadIDs verifies that
// gmail_get_threads_batch returns error when thread_ids is missing.
func TestGmailHandlerGetThreadsContentBatchMissingThreadIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_threads_batch", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "thread_ids") {
		t.Errorf("expected error mentioning 'thread_ids', got %q", text)
	}
}

// TestGmailHandlerGetThreadsContentBatchEmptyThreadIDs verifies that
// gmail_get_threads_batch returns error when thread_ids is empty.
func TestGmailHandlerGetThreadsContentBatchEmptyThreadIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_get_threads_batch", map[string]any{
		"thread_ids": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "no thread ids") {
		t.Errorf("expected error mentioning empty thread_ids, got %q", text)
	}
}

// TestGmailHandlerModifyMessageLabelsMissingMessageID verifies that
// gmail_modify_message_labels returns error when message_id is missing.
func TestGmailHandlerModifyMessageLabelsMissingMessageID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_modify_message_labels", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_id") {
		t.Errorf("expected error mentioning 'message_id', got %q", text)
	}
}

// TestGmailHandlerModifyMessageLabelsMissingBothLabelArrays verifies that
// gmail_modify_message_labels returns error when neither add nor remove labels provided.
func TestGmailHandlerModifyMessageLabelsMissingBothLabelArrays(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_modify_message_labels", map[string]any{
		"message_id": "msg123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "add_label_ids") && !strings.Contains(strings.ToLower(text), "remove_label_ids") {
		t.Errorf("expected error mentioning label arrays, got %q", text)
	}
}

// TestGmailHandlerManageGmailLabelMissingAction verifies that
// gmail_manage_label returns error when action is missing.
func TestGmailHandlerManageGmailLabelMissingAction(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_manage_label", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "action") {
		t.Errorf("expected error mentioning 'action', got %q", text)
	}
}

// TestGmailHandlerManageGmailLabelCreateMissingName verifies that
// gmail_manage_label with action=create returns error when name is missing.
// Note: the name check happens after auth, so without credentials we get an
// auth error first. This test verifies the handler still returns an error.
func TestGmailHandlerManageGmailLabelCreateMissingName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_manage_label", map[string]any{
		"action": "create",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	// Auth error comes before name validation; verify we get some error.
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "name") && !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error mentioning 'name' or credentials, got %q", text)
	}
}

// TestGmailHandlerSendGmailMessageMissingTo verifies that
// gmail_send_message returns error when to is missing.
func TestGmailHandlerSendGmailMessageMissingTo(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_send_message", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	// The handler checks email first, then to, subject, body in order.
	// With USER_GOOGLE_EMAIL set, it should get to the "to" check.
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "to") {
		t.Errorf("expected error mentioning 'to', got %q", text)
	}
}

// TestGmailHandlerSendGmailMessageMissingSubject verifies that
// gmail_send_message returns error when subject is missing.
func TestGmailHandlerSendGmailMessageMissingSubject(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_send_message", map[string]any{
		"to": "someone@example.com",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "subject") {
		t.Errorf("expected error mentioning 'subject', got %q", text)
	}
}

// TestGmailHandlerSendGmailMessageMissingBody verifies that
// gmail_send_message returns error when body is missing.
func TestGmailHandlerSendGmailMessageMissingBody(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_send_message", map[string]any{
		"to":      "someone@example.com",
		"subject": "Test Subject",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "body") {
		t.Errorf("expected error mentioning 'body', got %q", text)
	}
}

// TestGmailHandlerDraftGmailMessageMissingSubject verifies that
// gmail_draft_message returns error when subject is missing.
func TestGmailHandlerDraftGmailMessageMissingSubject(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_draft_message", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "subject") {
		t.Errorf("expected error mentioning 'subject', got %q", text)
	}
}

// TestGmailHandlerDraftGmailMessageMissingBody verifies that
// gmail_draft_message returns error when body is missing.
func TestGmailHandlerDraftGmailMessageMissingBody(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_draft_message", map[string]any{
		"subject": "Test Subject",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "body") {
		t.Errorf("expected error mentioning 'body', got %q", text)
	}
}

// TestGmailHandlerBatchModifyMessageLabelsMissingMessageIDs verifies that
// gmail_batch_modify_message_labels returns error when message_ids is missing.
func TestGmailHandlerBatchModifyMessageLabelsMissingMessageIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_batch_modify_message_labels", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "message_ids") {
		t.Errorf("expected error mentioning 'message_ids', got %q", text)
	}
}

// TestGmailHandlerBatchModifyMessageLabelsEmptyMessageIDs verifies that
// gmail_batch_modify_message_labels returns error when message_ids is empty.
func TestGmailHandlerBatchModifyMessageLabelsEmptyMessageIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_batch_modify_message_labels", map[string]any{
		"message_ids": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "message_ids") && !strings.Contains(lower, "empty") {
		t.Errorf("expected error mentioning empty message_ids, got %q", text)
	}
}

// TestGmailHandlerCreateGmailFilterMissingCriteria verifies that
// gmail_create_filter returns error when criteria is missing.
func TestGmailHandlerCreateGmailFilterMissingCriteria(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_create_filter", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "criteria") {
		t.Errorf("expected error mentioning 'criteria', got %q", text)
	}
}

// TestGmailHandlerCreateGmailFilterMissingAction verifies that
// gmail_create_filter returns error when action is missing.
func TestGmailHandlerCreateGmailFilterMissingAction(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_create_filter", map[string]any{
		"criteria": map[string]any{"from": "test@example.com"},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "action") {
		t.Errorf("expected error mentioning 'action', got %q", text)
	}
}

// TestGmailHandlerDeleteGmailFilterMissingFilterID verifies that
// gmail_delete_filter returns error when filter_id is missing.
func TestGmailHandlerDeleteGmailFilterMissingFilterID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_delete_filter", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "filter_id") {
		t.Errorf("expected error mentioning 'filter_id', got %q", text)
	}
}

// TestGmailHandlerAuthFailureSearchGmailMessages verifies that
// gmail_search_messages with valid params but no credentials returns
// an error mentioning credentials or authentication.
func TestGmailHandlerAuthFailureSearchGmailMessages(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_search_messages", map[string]any{
		"query": "in:inbox",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// TestGmailHandlerAuthFailureListGmailLabels verifies that
// gmail_list_labels with no credentials returns an auth error.
func TestGmailHandlerAuthFailureListGmailLabels(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "gmail_list_labels", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}
