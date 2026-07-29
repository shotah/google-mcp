package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// helper to call a handler and return the text content, failing on error.
func callHandlerOK(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
	t.Helper()
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		tc, _ := result.Content[0].(mcp.TextContent)
		t.Fatalf("handler returned tool error: %s", tc.Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("handler returned empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// helper to call a handler and return the error text, failing if no error.
func callHandlerErr(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
	t.Helper()
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned unexpected Go error: %v", err)
	}
	if !result.IsError {
		tc := result.Content[0].(mcp.TextContent)
		t.Fatalf("expected tool error, got success: %s", tc.Text)
	}
	tc := result.Content[0].(mcp.TextContent)
	return tc.Text
}

// --- gmail_search_messages ---

func TestGmailMockSearchMessages(t *testing.T) {
	t.Run("success_with_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages": map[string]any{
				"messages": []map[string]any{
					{"id": "msg001", "threadId": "thread001"},
					{"id": "msg002", "threadId": "thread002"},
				},
				"resultSizeEstimate": 2,
			},
		})
		handler := handleSearchGmailMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "in:inbox",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 messages") {
			t.Errorf("expected 'Found 2 messages', got:\n%s", text)
		}
		if !strings.Contains(text, "msg001") {
			t.Errorf("expected msg001 in output")
		}
		if !strings.Contains(text, "msg002") {
			t.Errorf("expected msg002 in output")
		}
		if !strings.Contains(text, "thread001") {
			t.Errorf("expected thread001 in output")
		}
	})

	t.Run("success_no_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages": map[string]any{
				"resultSizeEstimate": 0,
			},
		})
		handler := handleSearchGmailMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "from:nobody@example.com",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No messages found") {
			t.Errorf("expected 'No messages found', got:\n%s", text)
		}
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages": map[string]any{
				"messages": []map[string]any{
					{"id": "msg001", "threadId": "thread001"},
				},
				"resultSizeEstimate": 1,
				"nextPageToken":      "token123",
			},
		})
		handler := handleSearchGmailMessages(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "in:inbox",
			"user_google_email": "test@example.com",
			"page_size":         float64(1),
		})
		if !strings.Contains(text, "token123") {
			t.Errorf("expected pagination token in output, got:\n%s", text)
		}
	})
}

// --- gmail_get_message ---

func TestGmailMockGetMessageContent(t *testing.T) {
	// Base64url-encode a body for the Gmail API response.
	bodyText := "Hello, this is the message body."
	bodyB64 := base64.URLEncoding.EncodeToString([]byte(bodyText))

	t.Run("success_with_body_and_headers", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg001": map[string]any{
				"id":       "msg001",
				"threadId": "thread001",
				"payload": map[string]any{
					"headers": []map[string]any{
						{"name": "Subject", "value": "Test Email"},
						{"name": "From", "value": "alice@example.com"},
						{"name": "To", "value": "bob@example.com"},
						{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
						{"name": "Message-ID", "value": "<msg001@example.com>"},
					},
					"mimeType": "text/plain",
					"body": map[string]any{
						"data": bodyB64,
					},
				},
			},
		})
		handler := handleGetGmailMessageContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_id":        "msg001",
			"user_google_email": "test@example.com",
		})
		for _, want := range []string{"Subject: Test Email", "From:    alice@example.com", "To:      bob@example.com", bodyText, "Message-ID: <msg001@example.com>"} {
			if !strings.Contains(text, want) {
				t.Errorf("expected %q in output, got:\n%s", want, text)
			}
		}
	})

	t.Run("success_with_attachments", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg002": map[string]any{
				"id":       "msg002",
				"threadId": "thread002",
				"payload": map[string]any{
					"headers": []map[string]any{
						{"name": "Subject", "value": "With Attachment"},
						{"name": "From", "value": "alice@example.com"},
						{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
					},
					"mimeType": "multipart/mixed",
					"parts": []map[string]any{
						{
							"mimeType": "text/plain",
							"body": map[string]any{
								"data": bodyB64,
							},
						},
						{
							"filename": "report.pdf",
							"mimeType": "application/pdf",
							"body": map[string]any{
								"attachmentId": "att001",
								"size":         2048,
							},
						},
					},
				},
			},
		})
		handler := handleGetGmailMessageContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_id":        "msg002",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "ATTACHMENTS") {
			t.Errorf("expected ATTACHMENTS section in output")
		}
		if !strings.Contains(text, "report.pdf") {
			t.Errorf("expected 'report.pdf' in output")
		}
		if !strings.Contains(text, "att001") {
			t.Errorf("expected attachment ID 'att001' in output")
		}
	})
}

// --- gmail_get_messages_batch ---

func TestGmailMockGetMessagesContentBatch(t *testing.T) {
	bodyB64 := base64.URLEncoding.EncodeToString([]byte("Batch body 1"))

	t.Run("success_full_format", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg001": map[string]any{
				"id":       "msg001",
				"threadId": "thread001",
				"payload": map[string]any{
					"headers": []map[string]any{
						{"name": "Subject", "value": "First Message"},
						{"name": "From", "value": "alice@example.com"},
						{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
					},
					"mimeType": "text/plain",
					"body": map[string]any{
						"data": bodyB64,
					},
				},
			},
			"/gmail/v1/users/me/messages/msg002": map[string]any{
				"id":       "msg002",
				"threadId": "thread002",
				"payload": map[string]any{
					"headers": []map[string]any{
						{"name": "Subject", "value": "Second Message"},
						{"name": "From", "value": "bob@example.com"},
						{"name": "Date", "value": "Tue, 2 Jan 2026 14:00:00 +0000"},
					},
					"mimeType": "text/plain",
					"body": map[string]any{
						"data": bodyB64,
					},
				},
			},
		})
		handler := handleGetGmailMessagesContentBatch(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_ids":       []any{"msg001", "msg002"},
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Retrieved 2 messages") {
			t.Errorf("expected 'Retrieved 2 messages', got:\n%s", text)
		}
		if !strings.Contains(text, "First Message") {
			t.Errorf("expected 'First Message' in output")
		}
		if !strings.Contains(text, "Second Message") {
			t.Errorf("expected 'Second Message' in output")
		}
	})

	t.Run("success_metadata_format", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg001": map[string]any{
				"id":       "msg001",
				"threadId": "thread001",
				"payload": map[string]any{
					"headers": []map[string]any{
						{"name": "Subject", "value": "Metadata Only"},
						{"name": "From", "value": "alice@example.com"},
						{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
					},
				},
			},
		})
		handler := handleGetGmailMessagesContentBatch(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_ids":       []any{"msg001"},
			"format":            "metadata",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Retrieved 1 messages") {
			t.Errorf("expected 'Retrieved 1 messages', got:\n%s", text)
		}
		if !strings.Contains(text, "Metadata Only") {
			t.Errorf("expected 'Metadata Only' in output")
		}
	})
}

// --- gmail_send_message ---

func TestGmailMockSendMessage(t *testing.T) {
	t.Run("success_plain_text", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/send": map[string]any{
				"id":       "sent001",
				"threadId": "thread-sent-001",
			},
		})
		handler := handleSendGmailMessage(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"to":                "recipient@example.com",
			"subject":           "Test Subject",
			"body":              "Hello from tests!",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Email sent successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "sent001") {
			t.Errorf("expected sent message ID, got:\n%s", text)
		}
	})
}

// --- gmail_draft_message ---

func TestGmailMockDraftMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/drafts": map[string]any{
				"id": "draft001",
				"message": map[string]any{
					"id": "msg-draft-001",
				},
			},
		})
		handler := handleDraftGmailMessage(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"subject":           "Draft Subject",
			"body":              "Draft body content",
			"to":                "someone@example.com",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Draft created successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "draft001") {
			t.Errorf("expected draft ID in output, got:\n%s", text)
		}
	})

	t.Run("success_without_to", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/drafts": map[string]any{
				"id": "draft002",
				"message": map[string]any{
					"id": "msg-draft-002",
				},
			},
		})
		handler := handleDraftGmailMessage(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"subject":           "No Recipient Draft",
			"body":              "Just saving a draft",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Draft created successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
	})
}

// --- gmail_list_labels ---

func TestGmailMockListLabels(t *testing.T) {
	t.Run("success_with_labels", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/labels": map[string]any{
				"labels": []map[string]any{
					{"id": "INBOX", "name": "INBOX", "type": "system"},
					{"id": "SENT", "name": "SENT", "type": "system"},
					{"id": "Label_1", "name": "MyLabel", "type": "user"},
				},
			},
		})
		handler := handleListGmailLabels(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 3 labels") {
			t.Errorf("expected 'Found 3 labels', got:\n%s", text)
		}
		if !strings.Contains(text, "SYSTEM LABELS") {
			t.Errorf("expected 'SYSTEM LABELS' section")
		}
		if !strings.Contains(text, "USER LABELS") {
			t.Errorf("expected 'USER LABELS' section")
		}
		if !strings.Contains(text, "INBOX") {
			t.Errorf("expected 'INBOX' in output")
		}
		if !strings.Contains(text, "MyLabel") {
			t.Errorf("expected 'MyLabel' in output")
		}
	})

	t.Run("success_no_labels", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/labels": map[string]any{
				"labels": []map[string]any{},
			},
		})
		handler := handleListGmailLabels(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No labels found") {
			t.Errorf("expected 'No labels found', got:\n%s", text)
		}
	})
}

// --- API error responses ---

func TestGmailMockAPIError(t *testing.T) {
	t.Run("search_404_error", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages": `{"error": {"code": 404, "message": "Not Found", "status": "NOT_FOUND"}}`,
		})
		// Override to return 404 status code with error body.
		ts.Close()
		ts = fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/gmail/v1/users/me/messages": {code: 404, body: `{"error": {"code": 404, "message": "Not Found", "status": "NOT_FOUND"}}`},
		})
		handler := handleSearchGmailMessages(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"query":             "in:inbox",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Gmail API error") {
			t.Errorf("expected 'Gmail API error' in error message, got:\n%s", text)
		}
	})

	t.Run("get_message_404_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/gmail/v1/users/me/messages/nonexistent": {code: 404, body: `{"error": {"code": 404, "message": "Not Found"}}`},
		})
		handler := handleGetGmailMessageContent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"message_id":        "nonexistent",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Gmail API error") {
			t.Errorf("expected error message, got:\n%s", text)
		}
	})

	t.Run("send_message_server_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/gmail/v1/users/me/messages/send": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleSendGmailMessage(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"to":                "someone@example.com",
			"subject":           "Test",
			"body":              "Test body",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Gmail API error") {
			t.Errorf("expected error message, got:\n%s", text)
		}
	})

	t.Run("list_labels_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/gmail/v1/users/me/labels": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := handleListGmailLabels(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Gmail API error") {
			t.Errorf("expected error message, got:\n%s", text)
		}
	})
}

// --- gmail_get_thread ---

func TestGmailMockGetThreadContent(t *testing.T) {
	bodyB64 := base64.URLEncoding.EncodeToString([]byte("Thread message body"))

	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/threads/thread001": map[string]any{
				"id": "thread001",
				"messages": []map[string]any{
					{
						"id": "msg001",
						"payload": map[string]any{
							"headers": []map[string]any{
								{"name": "Subject", "value": "Thread Subject"},
								{"name": "From", "value": "alice@example.com"},
								{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
							},
							"mimeType": "text/plain",
							"body": map[string]any{
								"data": bodyB64,
							},
						},
					},
					{
						"id": "msg002",
						"payload": map[string]any{
							"headers": []map[string]any{
								{"name": "Subject", "value": "Re: Thread Subject"},
								{"name": "From", "value": "bob@example.com"},
								{"name": "Date", "value": "Tue, 2 Jan 2026 10:00:00 +0000"},
							},
							"mimeType": "text/plain",
							"body": map[string]any{
								"data": bodyB64,
							},
						},
					},
				},
			},
		})
		handler := handleGetGmailThreadContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"thread_id":         "thread001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Thread ID: thread001") {
			t.Errorf("expected thread ID in output")
		}
		if !strings.Contains(text, "Messages in thread: 2") {
			t.Errorf("expected 2 messages in thread, got:\n%s", text)
		}
		if !strings.Contains(text, "Message 1 of 2") {
			t.Errorf("expected 'Message 1 of 2' in output")
		}
		if !strings.Contains(text, "Message 2 of 2") {
			t.Errorf("expected 'Message 2 of 2' in output")
		}
	})
}

// --- gmail_get_threads_batch ---

func TestGmailMockGetThreadsContentBatch(t *testing.T) {
	bodyB64 := base64.URLEncoding.EncodeToString([]byte("Batch thread body"))

	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/threads/thread001": map[string]any{
				"id": "thread001",
				"messages": []map[string]any{
					{
						"id": "msg001",
						"payload": map[string]any{
							"headers": []map[string]any{
								{"name": "Subject", "value": "Thread 1"},
								{"name": "From", "value": "a@example.com"},
								{"name": "Date", "value": "Mon, 1 Jan 2026 12:00:00 +0000"},
							},
							"mimeType": "text/plain",
							"body":     map[string]any{"data": bodyB64},
						},
					},
				},
			},
			"/gmail/v1/users/me/threads/thread002": map[string]any{
				"id": "thread002",
				"messages": []map[string]any{
					{
						"id": "msg002",
						"payload": map[string]any{
							"headers": []map[string]any{
								{"name": "Subject", "value": "Thread 2"},
								{"name": "From", "value": "b@example.com"},
								{"name": "Date", "value": "Tue, 2 Jan 2026 12:00:00 +0000"},
							},
							"mimeType": "text/plain",
							"body":     map[string]any{"data": bodyB64},
						},
					},
				},
			},
		})
		handler := handleGetGmailThreadsContentBatch(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"thread_ids":        []any{"thread001", "thread002"},
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Retrieved 2 threads") {
			t.Errorf("expected 'Retrieved 2 threads', got:\n%s", text)
		}
		if !strings.Contains(text, "Thread 1") {
			t.Errorf("expected 'Thread 1' in output")
		}
		if !strings.Contains(text, "Thread 2") {
			t.Errorf("expected 'Thread 2' in output")
		}
	})
}

// --- gmail_manage_label ---

func TestGmailMockManageLabel(t *testing.T) {
	t.Run("create_success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/labels": map[string]any{
				"id":   "Label_new",
				"name": "NewLabel",
			},
		})
		handler := handleManageGmailLabel(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"action":            "create",
			"name":              "NewLabel",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Label created successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "NewLabel") {
			t.Errorf("expected label name in output")
		}
	})
}

// --- gmail_list_filters ---

func TestGmailMockListFilters(t *testing.T) {
	t.Run("success_with_filters", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/settings/filters": map[string]any{
				"filter": []map[string]any{
					{
						"id": "filter001",
						"criteria": map[string]any{
							"from": "news@example.com",
						},
						"action": map[string]any{
							"addLabelIds":    []string{"Label_1"},
							"removeLabelIds": []string{"INBOX"},
						},
					},
				},
			},
		})
		handler := handleListGmailFilters(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 1 filter") {
			t.Errorf("expected 'Found 1 filter', got:\n%s", text)
		}
		if !strings.Contains(text, "filter001") {
			t.Errorf("expected filter ID in output")
		}
		if !strings.Contains(text, "news@example.com") {
			t.Errorf("expected criteria in output")
		}
	})

	t.Run("success_no_filters", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/settings/filters": map[string]any{},
		})
		handler := handleListGmailFilters(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No filters found") {
			t.Errorf("expected 'No filters found', got:\n%s", text)
		}
	})
}

// --- gmail_create_filter ---

func TestGmailMockCreateFilter(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/settings/filters": map[string]any{
				"id": "filter_new",
			},
		})
		handler := handleCreateGmailFilter(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"criteria":          map[string]any{"from": "noreply@example.com"},
			"action":            map[string]any{"addLabelIds": []any{"Label_1"}},
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Filter created successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "filter_new") {
			t.Errorf("expected filter ID in output")
		}
	})
}

// --- gmail_delete_filter ---

func TestGmailMockDeleteFilter(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/settings/filters/filter001": map[string]any{
				"id": "filter001",
				"criteria": map[string]any{
					"from": "old@example.com",
				},
				"action": map[string]any{
					"forward": "archive@example.com",
				},
			},
		})
		handler := handleDeleteGmailFilter(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"filter_id":         "filter001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Filter deleted successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "filter001") {
			t.Errorf("expected filter ID in output")
		}
	})
}

// --- gmail_modify_message_labels ---

func TestGmailMockModifyMessageLabels(t *testing.T) {
	t.Run("success_add_and_remove", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg001/modify": map[string]any{
				"id": "msg001",
			},
		})
		handler := handleModifyGmailMessageLabels(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_id":        "msg001",
			"add_label_ids":     []any{"STARRED"},
			"remove_label_ids":  []any{"INBOX"},
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Labels modified for message msg001") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Added: STARRED") {
			t.Errorf("expected added labels in output")
		}
		if !strings.Contains(text, "Removed: INBOX") {
			t.Errorf("expected removed labels in output")
		}
	})
}

// --- gmail_batch_modify_message_labels ---

func TestGmailMockBatchModifyMessageLabels(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// BatchModify returns empty body on success (204 No Content equivalent).
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/batchModify": `{}`,
		})
		handler := handleBatchModifyGmailMessageLabels(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_ids":       []any{"msg001", "msg002"},
			"add_label_ids":     []any{"Label_1"},
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Labels modified for 2 messages") {
			t.Errorf("expected success message, got:\n%s", text)
		}
	})
}

// --- gmail_get_attachment ---

func TestGmailMockGetAttachmentContent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		attData := base64.URLEncoding.EncodeToString([]byte("fake attachment data"))
		ts := fakeAPIServer(t, map[string]any{
			"/gmail/v1/users/me/messages/msg001/attachments/att001": map[string]any{
				"size": 20,
				"data": attData,
			},
		})
		handler := handleGetGmailAttachmentContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"message_id":        "msg001",
			"attachment_id":     "att001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Attachment downloaded successfully") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "msg001") {
			t.Errorf("expected message ID in output")
		}
	})
}

// --- statusResponse and fakeAPIServerWithStatus ---
// Allows returning specific HTTP status codes for error testing.

type statusResponse struct {
	code int
	body string
}

func fakeAPIServerWithStatus(t *testing.T, routes map[string]statusResponse) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for prefix, resp := range routes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(resp.code)
				fmt.Fprint(w, resp.body)
				return
			}
		}
		t.Logf("fakeAPIServerWithStatus: unmatched path: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	return ts
}
