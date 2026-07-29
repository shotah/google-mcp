package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCommentsMockReadComments(t *testing.T) {
	t.Run("success_with_comments_and_replies", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/doc001/comments": `{
				"comments": [
					{
						"id":"comment001",
						"content":"Please update this section.",
						"author":{"displayName":"Alice"},
						"createdTime":"2026-02-18T10:00:00Z",
						"resolved":false,
						"replies":[
							{"id":"reply001","content":"Updated.","author":{"displayName":"Bob"},"createdTime":"2026-02-18T10:05:00Z"}
						]
					},
					{
						"id":"comment002",
						"content":"Looks good.",
						"author":{"displayName":"Carol"},
						"createdTime":"2026-02-18T11:00:00Z",
						"resolved":true
					}
				]
			}`,
		})
		handler := makeReadCommentsHandler(testClientFunc(ts), "docs", "document", "document_id")
		text := callHandlerOK(t, handler, map[string]any{
			"document_id":       "doc001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 comments in document doc001") {
			t.Errorf("expected comment count, got:\n%s", text)
		}
		if !strings.Contains(text, "Alice") || !strings.Contains(text, "Please update this section.") {
			t.Errorf("expected first comment details, got:\n%s", text)
		}
		if !strings.Contains(text, "reply001") || !strings.Contains(text, "Updated.") {
			t.Errorf("expected reply details, got:\n%s", text)
		}
		if !strings.Contains(text, "[RESOLVED]") {
			t.Errorf("expected resolved status, got:\n%s", text)
		}
	})

	t.Run("success_empty", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/doc001/comments": `{"comments":[]}`,
		})
		handler := makeReadCommentsHandler(testClientFunc(ts), "docs", "document", "document_id")
		text := callHandlerOK(t, handler, map[string]any{
			"document_id":       "doc001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No comments found in document doc001") {
			t.Errorf("expected empty comment result, got:\n%s", text)
		}
	})
}

func TestCommentsMockCreateComment(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/drive/v3/files/doc001/comments": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"comment001","content":"Please review this.","author":{"displayName":"Alice"},"createdTime":"2026-02-18T10:00:00Z"}`)
		},
	})
	handler := makeCreateCommentHandler(testClientFunc(ts), "docs", "document_id")
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"comment_content":   "Please review this.",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Comment created successfully!") || !strings.Contains(text, "comment001") {
		t.Errorf("expected create result, got:\n%s", text)
	}
	if !strings.Contains(text, "Alice") || !strings.Contains(text, "Please review this.") {
		t.Errorf("expected created comment details, got:\n%s", text)
	}
}

func TestCommentsMockReplyToComment(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/drive/v3/files/doc001/comments/comment001/replies": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"reply001","content":"I will update it.","author":{"displayName":"Bob"},"createdTime":"2026-02-18T10:05:00Z"}`)
		},
	})
	handler := makeReplyToCommentHandler(testClientFunc(ts), "docs", "document_id")
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"comment_id":        "comment001",
		"reply_content":     "I will update it.",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Reply posted successfully!") || !strings.Contains(text, "reply001") {
		t.Errorf("expected reply result, got:\n%s", text)
	}
	if !strings.Contains(text, "Bob") || !strings.Contains(text, "I will update it.") {
		t.Errorf("expected reply details, got:\n%s", text)
	}
}

func TestCommentsMockResolveComment(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/drive/v3/files/doc001/comments/comment001/replies": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"reply002","content":"This comment has been resolved.","author":{"displayName":"Alice"},"createdTime":"2026-02-18T10:10:00Z"}`)
		},
	})
	handler := makeResolveCommentHandler(testClientFunc(ts), "docs", "document_id")
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"comment_id":        "comment001",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Comment comment001 has been resolved successfully.") {
		t.Errorf("expected resolve result, got:\n%s", text)
	}
	if !strings.Contains(text, "reply002") || !strings.Contains(text, "Alice") {
		t.Errorf("expected resolve reply details, got:\n%s", text)
	}
}
