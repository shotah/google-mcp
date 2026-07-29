package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// calendarFakeServer creates a test server with longest-prefix-first matching
// for Calendar API routes. Supports func(http.ResponseWriter, *http.Request)
// route values for full request inspection.
func calendarFakeServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()
	return driveFakeServer(t, routes) // reuse longest-prefix-first matcher
}

// --- calendar_list_calendars ---

func TestCalendarMockListCalendars(t *testing.T) {
	t.Run("success_with_calendars", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/users/me/calendarList": map[string]any{
				"items": []map[string]any{
					{"id": "primary@example.com", "summary": "My Calendar", "primary": true},
					{"id": "work@group.calendar.google.com", "summary": "Work Calendar"},
				},
			},
		})
		handler := handleListCalendars(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully listed 2 calendars") {
			t.Errorf("expected 2 calendars listed, got:\n%s", text)
		}
		if !strings.Contains(text, "My Calendar") {
			t.Errorf("expected 'My Calendar' in output")
		}
		if !strings.Contains(text, "(Primary)") {
			t.Errorf("expected '(Primary)' marker in output")
		}
		if !strings.Contains(text, "Work Calendar") {
			t.Errorf("expected 'Work Calendar' in output")
		}
		if !strings.Contains(text, "primary@example.com") {
			t.Errorf("expected calendar ID in output")
		}
	})

	t.Run("success_no_calendars", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/users/me/calendarList": map[string]any{
				"items": []map[string]any{},
			},
		})
		handler := handleListCalendars(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No calendars found") {
			t.Errorf("expected 'No calendars found', got:\n%s", text)
		}
	})

	t.Run("success_no_summary", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/users/me/calendarList": map[string]any{
				"items": []map[string]any{
					{"id": "cal001", "summary": ""},
				},
			},
		})
		handler := handleListCalendars(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No Summary") {
			t.Errorf("expected 'No Summary' fallback, got:\n%s", text)
		}
	})
}

// --- calendar_list_events ---

func TestCalendarMockGetEvents(t *testing.T) {
	t.Run("success_with_time_range", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"items": []map[string]any{
					{
						"id":       "evt001",
						"summary":  "Team Standup",
						"start":    map[string]any{"dateTime": "2026-02-18T09:00:00Z"},
						"end":      map[string]any{"dateTime": "2026-02-18T09:30:00Z"},
						"htmlLink": "https://calendar.google.com/event?eid=evt001",
					},
					{
						"id":       "evt002",
						"summary":  "Lunch Break",
						"start":    map[string]any{"dateTime": "2026-02-18T12:00:00Z"},
						"end":      map[string]any{"dateTime": "2026-02-18T13:00:00Z"},
						"htmlLink": "https://calendar.google.com/event?eid=evt002",
					},
				},
			},
		})
		handler := handleListEvents(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"time_max":          "2026-02-19T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully retrieved 2 events") {
			t.Errorf("expected 2 events, got:\n%s", text)
		}
		if !strings.Contains(text, "Team Standup") {
			t.Errorf("expected 'Team Standup' in output")
		}
		if !strings.Contains(text, "Lunch Break") {
			t.Errorf("expected 'Lunch Break' in output")
		}
		if !strings.Contains(text, "evt001") {
			t.Errorf("expected event ID evt001 in output")
		}
	})

	t.Run("success_no_events", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"items": []map[string]any{},
			},
		})
		handler := handleListEvents(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"time_max":          "2026-02-18T01:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No events found") {
			t.Errorf("expected 'No events found', got:\n%s", text)
		}
	})

	t.Run("success_all_day_event", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"items": []map[string]any{
					{
						"id":       "evt003",
						"summary":  "Company Holiday",
						"start":    map[string]any{"date": "2026-02-20"},
						"end":      map[string]any{"date": "2026-02-21"},
						"htmlLink": "https://calendar.google.com/event?eid=evt003",
					},
				},
			},
		})
		handler := handleListEvents(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"time_min":          "2026-02-20",
			"time_max":          "2026-02-22",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Company Holiday") {
			t.Errorf("expected event in output, got:\n%s", text)
		}
		if !strings.Contains(text, "2026-02-20") {
			t.Errorf("expected all-day date in output")
		}
	})
}

// --- calendar_get_event ---

func TestCalendarMockGetEvent(t *testing.T) {
	t.Run("success_single_event_by_id", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events/evt001": map[string]any{
				"id":          "evt001",
				"summary":     "Important Meeting",
				"description": "Discuss roadmap",
				"location":    "Room 42",
				"start":       map[string]any{"dateTime": "2026-02-18T14:00:00Z"},
				"end":         map[string]any{"dateTime": "2026-02-18T15:00:00Z"},
				"htmlLink":    "https://calendar.google.com/event?eid=evt001",
			},
		})
		handler := handleGetEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"event_id":          "evt001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Important Meeting") {
			t.Errorf("expected event summary in output, got:\n%s", text)
		}
		if !strings.Contains(text, "evt001") {
			t.Errorf("expected event ID in output")
		}
		if !strings.Contains(text, "Event Details:") {
			t.Errorf("expected detailed single-event format, got:\n%s", text)
		}
	})

	t.Run("success_single_event_detailed", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events/evt001": map[string]any{
				"id":          "evt001",
				"summary":     "Detailed Event",
				"description": "Full details here",
				"location":    "Conference Room B",
				"start":       map[string]any{"dateTime": "2026-02-18T14:00:00Z"},
				"end":         map[string]any{"dateTime": "2026-02-18T15:00:00Z"},
				"htmlLink":    "https://calendar.google.com/event?eid=evt001",
				"attendees": []map[string]any{
					{"email": "alice@example.com", "responseStatus": "accepted"},
					{"email": "bob@example.com", "responseStatus": "tentative"},
				},
			},
		})
		handler := handleGetEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"event_id":          "evt001",
			"detailed":          true,
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Event Details:") {
			t.Errorf("expected detailed format, got:\n%s", text)
		}
		if !strings.Contains(text, "Full details here") {
			t.Errorf("expected description in output")
		}
		if !strings.Contains(text, "Conference Room B") {
			t.Errorf("expected location in output")
		}
		if !strings.Contains(text, "alice@example.com") {
			t.Errorf("expected attendee email in output")
		}
		if !strings.Contains(text, "accepted") {
			t.Errorf("expected response status in output")
		}
	})

	t.Run("error_bogus_event_id_primary", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{})
		handler := handleGetEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"event_id":          "primary",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "calendar_list_events") {
			t.Errorf("expected hint to use calendar_list_events, got:\n%s", text)
		}
	})

	t.Run("error_missing_event_id", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{})
		handler := handleGetEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "event_id is required") {
			t.Errorf("expected 'event_id is required', got:\n%s", text)
		}
	})
}

// --- calendar_create_event ---

func TestCalendarMockCreateEvent(t *testing.T) {
	t.Run("success_simple_event", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"id":       "new-evt-001",
				"summary":  "New Meeting",
				"htmlLink": "https://calendar.google.com/event?eid=new-evt-001",
			},
		})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"summary":           "New Meeting",
			"start_time":        "2026-02-20T10:00:00Z",
			"end_time":          "2026-02-20T11:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "New Meeting") {
			t.Errorf("expected event summary in output")
		}
		if !strings.Contains(text, "https://calendar.google.com/event") {
			t.Errorf("expected event link in output")
		}
	})

	t.Run("success_all_day_event", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"id":       "new-evt-002",
				"summary":  "Team Offsite",
				"htmlLink": "https://calendar.google.com/event?eid=new-evt-002",
			},
		})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"summary":           "Team Offsite",
			"start_time":        "2026-03-01",
			"end_time":          "2026-03-02",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Team Offsite") {
			t.Errorf("expected event summary in output")
		}
	})

	t.Run("success_with_attendees", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"id":       "new-evt-003",
				"summary":  "Planning Session",
				"htmlLink": "https://calendar.google.com/event?eid=new-evt-003",
			},
		})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"summary":           "Planning Session",
			"start_time":        "2026-02-25T14:00:00Z",
			"end_time":          "2026-02-25T16:00:00Z",
			"attendees":         []any{"alice@example.com", "bob@example.com"},
			"description":       "Quarterly planning",
			"location":          "Main Conference Room",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Planning Session") {
			t.Errorf("expected event summary in output")
		}
	})

	t.Run("success_with_google_meet", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events": map[string]any{
				"id":       "new-evt-004",
				"summary":  "Virtual Standup",
				"htmlLink": "https://calendar.google.com/event?eid=new-evt-004",
				"conferenceData": map[string]any{
					"entryPoints": []map[string]any{
						{
							"entryPointType": "video",
							"uri":            "https://meet.google.com/abc-defg-hij",
						},
					},
				},
			},
		})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"summary":           "Virtual Standup",
			"start_time":        "2026-02-20T09:00:00Z",
			"end_time":          "2026-02-20T09:15:00Z",
			"add_google_meet":   true,
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Google Meet") {
			t.Errorf("expected Google Meet link info in output, got:\n%s", text)
		}
		if !strings.Contains(text, "https://meet.google.com/abc-defg-hij") {
			t.Errorf("expected Meet URL in output, got:\n%s", text)
		}
	})

	t.Run("error_missing_summary", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"start_time":        "2026-02-20T10:00:00Z",
			"end_time":          "2026-02-20T11:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "summary is required") {
			t.Errorf("expected 'summary is required', got:\n%s", text)
		}
	})
}

// --- calendar_update_event ---

func TestCalendarMockModifyEvent(t *testing.T) {
	t.Run("success_update_summary", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events/evt001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					// Parse the request body to echo back the modified summary.
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					summary, _ := body["summary"].(string)
					if summary == "" {
						summary = "Updated Event"
					}
					fmt.Fprintf(w, `{"id":"evt001","summary":"%s","htmlLink":"https://calendar.google.com/event?eid=evt001"}`, summary)
				} else {
					// GET returns the existing event.
					fmt.Fprint(w, `{
						"id":"evt001","summary":"Original Event",
						"start":{"dateTime":"2026-02-18T14:00:00Z"},
						"end":{"dateTime":"2026-02-18T15:00:00Z"},
						"htmlLink":"https://calendar.google.com/event?eid=evt001"
					}`)
				}
			},
		})
		handler := handleModifyEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"event_id":          "evt001",
			"summary":           "Updated Event",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully modified event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Updated Event") {
			t.Errorf("expected updated summary in output, got:\n%s", text)
		}
		if !strings.Contains(text, "evt001") {
			t.Errorf("expected event ID in output")
		}
	})

	t.Run("success_update_time_and_location", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events/evt002": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					fmt.Fprint(w, `{
						"id":"evt002","summary":"Rescheduled Meeting",
						"htmlLink":"https://calendar.google.com/event?eid=evt002",
						"location":"Room 101"
					}`)
				} else {
					fmt.Fprint(w, `{
						"id":"evt002","summary":"Rescheduled Meeting",
						"start":{"dateTime":"2026-02-18T10:00:00Z"},
						"end":{"dateTime":"2026-02-18T11:00:00Z"},
						"htmlLink":"https://calendar.google.com/event?eid=evt002"
					}`)
				}
			},
		})
		handler := handleModifyEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"event_id":          "evt002",
			"start_time":        "2026-02-19T14:00:00Z",
			"end_time":          "2026-02-19T15:00:00Z",
			"location":          "Room 101",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully modified event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Rescheduled Meeting") {
			t.Errorf("expected event summary in output, got:\n%s", text)
		}
	})

	t.Run("error_missing_event_id", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleModifyEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"summary":           "Updated",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "event_id is required") {
			t.Errorf("expected 'event_id is required', got:\n%s", text)
		}
	})
}

// --- calendar_delete_event ---

func TestCalendarMockDeleteEvent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := calendarFakeServer(t, map[string]any{
			"/calendar/v3/calendars/primary/events/evt001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
				} else {
					// GET returns the event (existence check).
					fmt.Fprint(w, `{
						"id":"evt001","summary":"Event To Delete",
						"start":{"dateTime":"2026-02-18T14:00:00Z"},
						"end":{"dateTime":"2026-02-18T15:00:00Z"}
					}`)
				}
			},
		})
		handler := handleDeleteEvent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"event_id":          "evt001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully deleted event") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "evt001") {
			t.Errorf("expected event ID in output")
		}
	})

	t.Run("error_event_not_found", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/calendars/primary/events/nonexistent": {code: 404, body: `{"error": {"code": 404, "message": "Not Found"}}`},
		})
		handler := handleDeleteEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"event_id":          "nonexistent",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Event not found") {
			t.Errorf("expected 'Event not found' error, got:\n%s", text)
		}
	})

	t.Run("error_missing_event_id", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleDeleteEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "event_id is required") {
			t.Errorf("expected 'event_id is required', got:\n%s", text)
		}
	})
}

// --- calendar_query_freebusy ---

func TestCalendarMockQueryFreebusy(t *testing.T) {
	t.Run("success_with_busy_periods", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/freeBusy": map[string]any{
				"timeMin": "2026-02-18T00:00:00Z",
				"timeMax": "2026-02-19T00:00:00Z",
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{
							{"start": "2026-02-18T09:00:00Z", "end": "2026-02-18T10:00:00Z"},
							{"start": "2026-02-18T14:00:00Z", "end": "2026-02-18T15:00:00Z"},
						},
					},
				},
			},
		})
		handler := handleQueryFreebusy(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"time_max":          "2026-02-19T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Free/Busy information") {
			t.Errorf("expected free/busy header, got:\n%s", text)
		}
		if !strings.Contains(text, "Busy periods: 2") {
			t.Errorf("expected 2 busy periods, got:\n%s", text)
		}
		if !strings.Contains(text, "2026-02-18T09:00:00Z") {
			t.Errorf("expected busy period start time in output")
		}
	})

	t.Run("success_free_period", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/calendar/v3/freeBusy": map[string]any{
				"timeMin": "2026-02-18T00:00:00Z",
				"timeMax": "2026-02-19T00:00:00Z",
				"calendars": map[string]any{
					"primary": map[string]any{
						"busy": []map[string]any{},
					},
				},
			},
		})
		handler := handleQueryFreebusy(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"time_max":          "2026-02-19T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Free (no busy periods)") {
			t.Errorf("expected free status, got:\n%s", text)
		}
	})

	t.Run("error_missing_time_min", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleQueryFreebusy(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"time_max":          "2026-02-19T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "time_min is required") {
			t.Errorf("expected 'time_min is required', got:\n%s", text)
		}
	})
}

// --- API error responses ---

func TestCalendarMockAPIError(t *testing.T) {
	t.Run("list_calendars_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/users/me/calendarList": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := handleListCalendars(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "listing calendars") {
			t.Errorf("expected calendar listing error, got:\n%s", text)
		}
	})

	t.Run("get_events_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/calendars/primary/events": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleListEvents(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "listing events") {
			t.Errorf("expected events listing error, got:\n%s", text)
		}
	})

	t.Run("create_event_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/calendars/primary/events": {code: 400, body: `{"error": {"code": 400, "message": "Bad Request"}}`},
		})
		handler := handleCreateEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"summary":           "Bad Event",
			"start_time":        "invalid-time",
			"end_time":          "invalid-time",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "creating event") {
			t.Errorf("expected creating event error, got:\n%s", text)
		}
	})

	t.Run("modify_event_not_found", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/calendars/primary/events/missing": {code: 404, body: `{"error": {"code": 404, "message": "Not Found"}}`},
		})
		handler := handleModifyEvent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"event_id":          "missing",
			"summary":           "Updated",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "event not found") {
			t.Errorf("expected event not found error, got:\n%s", text)
		}
	})

	t.Run("freebusy_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/calendar/v3/freeBusy": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleQueryFreebusy(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"time_min":          "2026-02-18T00:00:00Z",
			"time_max":          "2026-02-19T00:00:00Z",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "querying free/busy") {
			t.Errorf("expected freebusy error, got:\n%s", text)
		}
	})
}
