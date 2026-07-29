package tools

import (
	"strings"
	"testing"
)

// --- forms_create ---

func TestFormsHandlerCreateFormMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_create", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

func TestFormsHandlerCreateFormAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_create", map[string]any{
		"title": "Test Form",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- forms_get ---

func TestFormsHandlerGetFormMissingFormID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_get", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "form_id") {
		t.Errorf("expected error mentioning 'form_id', got %q", text)
	}
}

func TestFormsHandlerGetFormAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_get", map[string]any{
		"form_id": "form123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- forms_set_publish_settings ---

func TestFormsHandlerSetPublishSettingsMissingFormID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_set_publish_settings", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "form_id") {
		t.Errorf("expected error mentioning 'form_id', got %q", text)
	}
}

// --- forms_get_response ---

func TestFormsHandlerGetFormResponseMissingFormID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_get_response", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "form_id") {
		t.Errorf("expected error mentioning 'form_id', got %q", text)
	}
}

func TestFormsHandlerGetFormResponseMissingResponseID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_get_response", map[string]any{
		"form_id": "form123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "response_id") {
		t.Errorf("expected error mentioning 'response_id', got %q", text)
	}
}

// --- forms_list_responses ---

func TestFormsHandlerListFormResponsesMissingFormID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_list_responses", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "form_id") {
		t.Errorf("expected error mentioning 'form_id', got %q", text)
	}
}

// --- forms_batch_update ---

func TestFormsHandlerBatchUpdateFormMissingFormID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_batch_update", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "form_id") {
		t.Errorf("expected error mentioning 'form_id', got %q", text)
	}
}

func TestFormsHandlerBatchUpdateFormMissingRequests(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_batch_update", map[string]any{
		"form_id": "form123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "requests") {
		t.Errorf("expected error mentioning 'requests', got %q", text)
	}
}

func TestFormsHandlerBatchUpdateFormRequestsNotArray(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "forms_batch_update", map[string]any{
		"form_id":  "form123",
		"requests": "not-an-array",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "requests") && !strings.Contains(lower, "array") {
		t.Errorf("expected error mentioning 'requests' or 'array', got %q", text)
	}
}
