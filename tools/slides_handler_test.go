package tools

import (
	"strings"
	"testing"
)

// --- slides_create_presentation ---
// slides_create_presentation has no strictly required params (title defaults,
// email resolved via env), so the first error path is auth failure.

func TestSlidesHandlerCreatePresentationAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_create_presentation", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- slides_get_presentation ---

func TestSlidesHandlerGetPresentationMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_presentation", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerGetPresentationAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_presentation", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- slides_batch_update ---

func TestSlidesHandlerBatchUpdatePresentationMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_batch_update", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerBatchUpdatePresentationMissingRequests(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_batch_update", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "requests") {
		t.Errorf("expected error mentioning 'requests', got %q", text)
	}
}

func TestSlidesHandlerBatchUpdatePresentationRequestsNotArray(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_batch_update", map[string]any{
		"presentation_id": "pres123",
		"requests":        "not-an-array",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "requests") && !strings.Contains(lower, "array") {
		t.Errorf("expected error mentioning 'requests' or 'array', got %q", text)
	}
}

// --- slides_get_page ---

func TestSlidesHandlerGetPageMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_page", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerGetPageMissingPageObjectID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_page", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "page_object_id") {
		t.Errorf("expected error mentioning 'page_object_id', got %q", text)
	}
}

// --- slides_get_page_thumbnail ---

func TestSlidesHandlerGetPageThumbnailMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_page_thumbnail", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerGetPageThumbnailMissingPageObjectID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_get_page_thumbnail", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "page_object_id") {
		t.Errorf("expected error mentioning 'page_object_id', got %q", text)
	}
}

// --- Presentation Comment Tools (via RegisterCommentTools) ---

func TestSlidesHandlerReadPresentationCommentsMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_read_comments", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerCreatePresentationCommentMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_create_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerCreatePresentationCommentMissingContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_create_comment", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_content") {
		t.Errorf("expected error mentioning 'comment_content', got %q", text)
	}
}

func TestSlidesHandlerReplyToPresentationCommentMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_reply_to_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerReplyToPresentationCommentMissingCommentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_reply_to_comment", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_id") {
		t.Errorf("expected error mentioning 'comment_id', got %q", text)
	}
}

func TestSlidesHandlerReplyToPresentationCommentMissingReplyContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_reply_to_comment", map[string]any{
		"presentation_id": "pres123",
		"comment_id":      "comment123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "reply_content") {
		t.Errorf("expected error mentioning 'reply_content', got %q", text)
	}
}

func TestSlidesHandlerResolvePresentationCommentMissingPresentationID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_resolve_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "presentation_id") {
		t.Errorf("expected error mentioning 'presentation_id', got %q", text)
	}
}

func TestSlidesHandlerResolvePresentationCommentMissingCommentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "slides_resolve_comment", map[string]any{
		"presentation_id": "pres123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_id") {
		t.Errorf("expected error mentioning 'comment_id', got %q", text)
	}
}
