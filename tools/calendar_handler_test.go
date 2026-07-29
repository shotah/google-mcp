package tools

import (
	"strings"
	"testing"
)

// --- calendar_list_calendars ---
// calendar_list_calendars has no strictly required params (email resolved via env),
// so the first error path is auth failure.

func TestCalendarHandlerListCalendarsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_list_calendars", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- calendar_list_events ---
// calendar_list_events has no strictly required params (defaults to primary calendar),
// so the first error path is auth failure.

func TestCalendarHandlerGetEventsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_list_events", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- calendar_query_freebusy ---

func TestCalendarHandlerQueryFreebusyMissingTimeMin(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_query_freebusy", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "time_min") {
		t.Errorf("expected error mentioning 'time_min', got %q", text)
	}
}

func TestCalendarHandlerQueryFreebusyMissingTimeMax(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_query_freebusy", map[string]any{
		"time_min": "2026-01-01T00:00:00Z",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "time_max") {
		t.Errorf("expected error mentioning 'time_max', got %q", text)
	}
}

func TestCalendarHandlerQueryFreebusyAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_query_freebusy", map[string]any{
		"time_min": "2026-01-01T00:00:00Z",
		"time_max": "2026-01-02T00:00:00Z",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- calendar_create_event ---

func TestCalendarHandlerCreateEventMissingSummary(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_create_event", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "summary") {
		t.Errorf("expected error mentioning 'summary', got %q", text)
	}
}

func TestCalendarHandlerCreateEventMissingStartTime(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_create_event", map[string]any{
		"summary": "Test Event",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "start_time") {
		t.Errorf("expected error mentioning 'start_time', got %q", text)
	}
}

func TestCalendarHandlerCreateEventMissingEndTime(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_create_event", map[string]any{
		"summary":    "Test Event",
		"start_time": "2026-01-01T10:00:00Z",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "end_time") {
		t.Errorf("expected error mentioning 'end_time', got %q", text)
	}
}

func TestCalendarHandlerCreateEventAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_create_event", map[string]any{
		"summary":    "Test Event",
		"start_time": "2026-01-01T10:00:00Z",
		"end_time":   "2026-01-01T11:00:00Z",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- calendar_update_event ---

func TestCalendarHandlerModifyEventMissingEventID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_update_event", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "event_id") {
		t.Errorf("expected error mentioning 'event_id', got %q", text)
	}
}

func TestCalendarHandlerModifyEventAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_update_event", map[string]any{
		"event_id": "evt123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- calendar_delete_event ---

func TestCalendarHandlerDeleteEventMissingEventID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_delete_event", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "event_id") {
		t.Errorf("expected error mentioning 'event_id', got %q", text)
	}
}

func TestCalendarHandlerDeleteEventAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "calendar_delete_event", map[string]any{
		"event_id": "evt123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}
