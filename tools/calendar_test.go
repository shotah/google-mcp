package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	calendar "google.golang.org/api/calendar/v3"
)

func TestNormalizeGetEventsIDs(t *testing.T) {
	cal, eid := normalizeGetEventsIDs("primary", "primary")
	if cal != "primary" || eid != "" {
		t.Fatalf("got calendar=%q event=%q", cal, eid)
	}
	cal, eid = normalizeGetEventsIDs("primary", "2026-07-28T00:00:00-07:00")
	if eid != "" || cal != "primary" {
		t.Fatalf("datetime event_id should clear: calendar=%q event=%q", cal, eid)
	}
	cal, eid = normalizeGetEventsIDs("primary", "realEventId123")
	if eid != "realEventId123" || cal != "primary" {
		t.Fatalf("real id should keep: calendar=%q event=%q", cal, eid)
	}
}

// --- correctTimeFormatForAPI ---

func TestCalendarCorrectTimeFormatForAPI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "already correct RFC3339", input: "2024-05-12T10:00:00Z", want: "2024-05-12T10:00:00Z"},
		{name: "bare date YYYY-MM-DD", input: "2024-05-12", want: "2024-05-12T00:00:00Z"},
		{name: "datetime without timezone", input: "2024-05-12T10:00:00", want: "2024-05-12T10:00:00Z"},
		{name: "datetime with positive offset", input: "2024-05-12T10:00:00+05:00", want: "2024-05-12T10:00:00+05:00"},
		{name: "datetime with negative offset", input: "2024-05-12T10:00:00-07:00", want: "2024-05-12T10:00:00-07:00"},
		{name: "datetime with Z timezone", input: "2024-01-01T00:00:00Z", want: "2024-01-01T00:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := correctTimeFormatForAPI(tt.input)
			if got != tt.want {
				t.Errorf("correctTimeFormatForAPI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- isAllDay ---

func TestCalendarIsAllDay(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "date-only format", input: "2024-05-12", want: true},
		{name: "datetime format", input: "2024-05-12T10:00:00Z", want: false},
		{name: "datetime with offset", input: "2024-05-12T10:00:00-07:00", want: false},
		{name: "empty string", input: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllDay(tt.input)
			if got != tt.want {
				t.Errorf("isAllDay(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- eventTime ---

func TestCalendarEventTime(t *testing.T) {
	tests := []struct {
		name string
		edt  *calendar.EventDateTime
		want string
	}{
		{name: "nil input", edt: nil, want: "Unknown"},
		{name: "dateTime set", edt: &calendar.EventDateTime{DateTime: "2024-05-12T10:00:00Z"}, want: "2024-05-12T10:00:00Z"},
		{name: "date set", edt: &calendar.EventDateTime{Date: "2024-05-12"}, want: "2024-05-12"},
		{name: "both set prefers dateTime", edt: &calendar.EventDateTime{DateTime: "2024-05-12T10:00:00Z", Date: "2024-05-12"}, want: "2024-05-12T10:00:00Z"},
		{name: "neither set", edt: &calendar.EventDateTime{}, want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eventTime(tt.edt)
			if got != tt.want {
				t.Errorf("eventTime(%+v) = %q, want %q", tt.edt, got, tt.want)
			}
		})
	}
}

// --- formatAttendeeEmails ---

func TestCalendarFormatAttendeeEmails(t *testing.T) {
	tests := []struct {
		name      string
		attendees []*calendar.EventAttendee
		want      string
	}{
		{name: "empty slice", attendees: []*calendar.EventAttendee{}, want: "None"},
		{name: "nil slice", attendees: nil, want: "None"},
		{
			name: "single attendee",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com"},
			},
			want: "alice@example.com",
		},
		{
			name: "multiple attendees",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com"},
				{Email: "bob@example.com"},
				{Email: "charlie@example.com"},
			},
			want: "alice@example.com, bob@example.com, charlie@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttendeeEmails(tt.attendees)
			if got != tt.want {
				t.Errorf("formatAttendeeEmails() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- formatAttendeeDetails ---

func TestCalendarFormatAttendeeDetails(t *testing.T) {
	tests := []struct {
		name         string
		attendees    []*calendar.EventAttendee
		indent       string
		wantContains []string
		wantExact    string
	}{
		{name: "empty slice", attendees: []*calendar.EventAttendee{}, indent: "  ", wantExact: "None"},
		{name: "nil slice", attendees: nil, indent: "  ", wantExact: "None"},
		{
			name: "with response status",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com", ResponseStatus: "accepted"},
			},
			indent: "  ",
			wantContains: []string{
				"alice@example.com",
				"accepted",
			},
		},
		{
			name: "default needsAction when empty status",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com"},
			},
			indent: "  ",
			wantContains: []string{
				"alice@example.com",
				"needsAction",
			},
		},
		{
			name: "organizer flag",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com", ResponseStatus: "accepted", Organizer: true},
			},
			indent: "  ",
			wantContains: []string{
				"alice@example.com",
				"[organizer]",
			},
		},
		{
			name: "optional flag",
			attendees: []*calendar.EventAttendee{
				{Email: "bob@example.com", ResponseStatus: "tentative", Optional: true},
			},
			indent: "  ",
			wantContains: []string{
				"bob@example.com",
				"[optional]",
			},
		},
		{
			name: "multiple attendees",
			attendees: []*calendar.EventAttendee{
				{Email: "alice@example.com", ResponseStatus: "accepted", Organizer: true},
				{Email: "bob@example.com", ResponseStatus: "declined"},
			},
			indent: "    ",
			wantContains: []string{
				"alice@example.com (accepted) [organizer]",
				"bob@example.com (declined)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAttendeeDetails(tt.attendees, tt.indent)
			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("formatAttendeeDetails() = %q, want %q", got, tt.wantExact)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatAttendeeDetails() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// --- formatEventAttachmentDetails ---

func TestCalendarFormatEventAttachmentDetails(t *testing.T) {
	tests := []struct {
		name         string
		attachments  []*calendar.EventAttachment
		indent       string
		wantContains []string
		wantExact    string
	}{
		{name: "empty slice", attachments: []*calendar.EventAttachment{}, indent: "  ", wantExact: "None"},
		{name: "nil slice", attachments: nil, indent: "  ", wantExact: "None"},
		{
			name: "with attachments",
			attachments: []*calendar.EventAttachment{
				{Title: "Report.pdf", FileUrl: "https://drive.google.com/open?id=abc123", MimeType: "application/pdf", FileId: "abc123"},
			},
			indent: "  ",
			wantContains: []string{
				"Report.pdf",
				"https://drive.google.com/open?id=abc123",
				"application/pdf",
				"abc123",
			},
		},
		{
			name: "untitled attachment",
			attachments: []*calendar.EventAttachment{
				{FileUrl: "https://example.com/file", MimeType: "text/plain"},
			},
			indent: "  ",
			wantContains: []string{
				"Untitled",
				"https://example.com/file",
			},
		},
		{
			name: "multiple attachments",
			attachments: []*calendar.EventAttachment{
				{Title: "File1.pdf", FileUrl: "https://example.com/1", MimeType: "application/pdf", FileId: "id1"},
				{Title: "File2.docx", FileUrl: "https://example.com/2", MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", FileId: "id2"},
			},
			indent: "    ",
			wantContains: []string{
				"File1.pdf",
				"File2.docx",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEventAttachmentDetails(tt.attachments, tt.indent)
			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("formatEventAttachmentDetails() = %q, want %q", got, tt.wantExact)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatEventAttachmentDetails() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

// --- formatDetailedSingleEvent ---

func TestCalendarFormatDetailedSingleEvent(t *testing.T) {
	tests := []struct {
		name               string
		event              *calendar.Event
		eventID            string
		includeAttachments bool
		wantContains       []string
	}{
		{
			name: "minimal event",
			event: &calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2024-05-12T10:00:00Z"},
				End:   &calendar.EventDateTime{DateTime: "2024-05-12T11:00:00Z"},
			},
			eventID: "evt1",
			wantContains: []string{
				"No Title",
				"2024-05-12T10:00:00Z",
				"2024-05-12T11:00:00Z",
				"No Description",
				"No Location",
				"evt1",
			},
		},
		{
			name: "full event with attendees",
			event: &calendar.Event{
				Summary:     "Team Meeting",
				Description: "Weekly sync",
				Location:    "Conference Room A",
				HtmlLink:    "https://calendar.google.com/event?id=abc",
				ColorId:     "5",
				Start:       &calendar.EventDateTime{DateTime: "2024-05-12T10:00:00Z"},
				End:         &calendar.EventDateTime{DateTime: "2024-05-12T11:00:00Z"},
				Attendees: []*calendar.EventAttendee{
					{Email: "alice@example.com", ResponseStatus: "accepted", Organizer: true},
					{Email: "bob@example.com", ResponseStatus: "tentative"},
				},
			},
			eventID: "evt2",
			wantContains: []string{
				"Team Meeting",
				"Weekly sync",
				"Conference Room A",
				"alice@example.com, bob@example.com",
				"accepted",
				"[organizer]",
				"evt2",
				"https://calendar.google.com/event?id=abc",
				"Color ID: 5",
			},
		},
		{
			name: "with attachments enabled",
			event: &calendar.Event{
				Summary: "Design Review",
				Start:   &calendar.EventDateTime{DateTime: "2024-05-12T14:00:00Z"},
				End:     &calendar.EventDateTime{DateTime: "2024-05-12T15:00:00Z"},
				Attachments: []*calendar.EventAttachment{
					{Title: "Mockup.pdf", FileUrl: "https://drive.google.com/open?id=xyz", MimeType: "application/pdf", FileId: "xyz"},
				},
			},
			eventID:            "evt3",
			includeAttachments: true,
			wantContains: []string{
				"Design Review",
				"Mockup.pdf",
				"application/pdf",
				"Attachments:",
			},
		},
		{
			name: "with attachments disabled",
			event: &calendar.Event{
				Summary: "Secret Meeting",
				Start:   &calendar.EventDateTime{DateTime: "2024-05-12T14:00:00Z"},
				End:     &calendar.EventDateTime{DateTime: "2024-05-12T15:00:00Z"},
				Attachments: []*calendar.EventAttachment{
					{Title: "Secret.pdf", FileUrl: "https://example.com/secret"},
				},
			},
			eventID:            "evt4",
			includeAttachments: false,
			wantContains: []string{
				"Secret Meeting",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDetailedSingleEvent(tt.event, tt.eventID, tt.includeAttachments)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatDetailedSingleEvent() = %q, should contain %q", got, want)
				}
			}
		})
	}

	// Extra: verify attachments NOT present when disabled
	t.Run("attachments not in output when disabled", func(t *testing.T) {
		event := &calendar.Event{
			Summary: "Test",
			Start:   &calendar.EventDateTime{DateTime: "2024-05-12T14:00:00Z"},
			End:     &calendar.EventDateTime{DateTime: "2024-05-12T15:00:00Z"},
			Attachments: []*calendar.EventAttachment{
				{Title: "Excluded.pdf"},
			},
		}
		got := formatDetailedSingleEvent(event, "evt5", false)
		if strings.Contains(got, "Excluded.pdf") {
			t.Errorf("formatDetailedSingleEvent() with includeAttachments=false should not contain attachment title, got %q", got)
		}
	})
}

// --- parseRemindersJSON ---

func TestCalendarParseRemindersJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantErrStr  string
		wantMethod  string
		wantMinutes int64
	}{
		{
			name:        "valid single popup reminder",
			input:       `[{"method": "popup", "minutes": 15}]`,
			wantCount:   1,
			wantMethod:  "popup",
			wantMinutes: 15,
		},
		{
			name:      "valid email reminder",
			input:     `[{"method": "email", "minutes": 30}]`,
			wantCount: 1,
		},
		{
			name:       "invalid JSON",
			input:      `not json`,
			wantErrStr: "invalid reminders JSON",
		},
		{
			name:       "empty JSON array",
			input:      `[]`,
			wantCount:  0,
			wantErrStr: "",
		},
		{
			name:       "too many reminders",
			input:      `[{"method":"popup","minutes":5},{"method":"popup","minutes":10},{"method":"popup","minutes":15},{"method":"popup","minutes":20},{"method":"popup","minutes":25},{"method":"popup","minutes":30}]`,
			wantErrStr: "maximum 5 reminders",
		},
		{
			name:       "invalid method",
			input:      `[{"method": "sms", "minutes": 10}]`,
			wantErrStr: "invalid reminder method",
		},
		{
			name:       "missing minutes",
			input:      `[{"method": "popup"}]`,
			wantErrStr: "must have 'minutes' field",
		},
		{
			name:       "minutes out of range negative",
			input:      `[{"method": "popup", "minutes": -1}]`,
			wantErrStr: "minutes must be between 0 and 40320",
		},
		{
			name:       "minutes out of range too high",
			input:      `[{"method": "popup", "minutes": 50000}]`,
			wantErrStr: "minutes must be between 0 and 40320",
		},
		{
			name:      "minutes at zero boundary",
			input:     `[{"method": "popup", "minutes": 0}]`,
			wantCount: 1,
		},
		{
			name:      "minutes at max boundary",
			input:     `[{"method": "popup", "minutes": 40320}]`,
			wantCount: 1,
		},
		{
			name:      "multiple valid reminders",
			input:     `[{"method": "popup", "minutes": 10}, {"method": "email", "minutes": 30}]`,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errMsg := parseRemindersJSON(tt.input)
			if tt.wantErrStr != "" {
				if errMsg == "" {
					t.Fatalf("expected error containing %q, got no error", tt.wantErrStr)
				}
				if !strings.Contains(errMsg, tt.wantErrStr) {
					t.Errorf("error = %q, should contain %q", errMsg, tt.wantErrStr)
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error: %s", errMsg)
			}
			if len(result) != tt.wantCount {
				t.Fatalf("got %d reminders, want %d", len(result), tt.wantCount)
			}
			if tt.wantMethod != "" && len(result) > 0 {
				if result[0].Method != tt.wantMethod {
					t.Errorf("method = %q, want %q", result[0].Method, tt.wantMethod)
				}
			}
			if tt.wantMinutes != 0 && len(result) > 0 {
				if result[0].Minutes != tt.wantMinutes {
					t.Errorf("minutes = %d, want %d", result[0].Minutes, tt.wantMinutes)
				}
			}
		})
	}
}

// --- extractDriveFileID ---

func TestCalendarExtractDriveFileID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw file ID", input: "abc123xyz", want: "abc123xyz"},
		{name: "full Drive URL with /d/", input: "https://drive.google.com/file/d/abc123/view?usp=sharing", want: "abc123"},
		{name: "short Drive URL with /d/", input: "https://drive.google.com/d/def456/edit", want: "def456"},
		{name: "Drive URL with id= param", input: "https://drive.google.com/open?id=ghi789", want: "ghi789"},
		{name: "Drive URL with id= and extra params", input: "https://drive.google.com/open?id=jkl012&authuser=0", want: "jkl012"},
		{name: "URL without file ID pattern", input: "https://example.com/some/page", want: ""},
		{name: "Drive URL /d/ no trailing slash", input: "https://drive.google.com/file/d/mno345", want: "mno345"},
		{name: "empty string", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDriveFileID(tt.input)
			if got != tt.want {
				t.Errorf("extractDriveFileID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEventDateTime(t *testing.T) {
	t.Parallel()
	allDay := eventDateTime("2026-07-23", "America/Los_Angeles")
	if allDay.Date != "2026-07-23" || allDay.DateTime != "" {
		t.Fatalf("all-day: %+v", allDay)
	}
	timed := eventDateTime("2026-07-23T10:00:00Z", "UTC")
	if timed.DateTime != "2026-07-23T10:00:00Z" || timed.TimeZone != "UTC" || timed.Date != "" {
		t.Fatalf("timed: %+v", timed)
	}
}

func TestBuildEventReminders(t *testing.T) {
	t.Parallel()
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]any{
				"use_default_reminders": false,
			},
		},
	}

	got, errMsg := buildEventReminders(req, nil, "", true)
	if errMsg != "" || got == nil || got.UseDefault {
		t.Fatalf("use_default=false: got=%+v err=%q", got, errMsg)
	}

	got, errMsg = buildEventReminders(req, &calendar.EventReminders{UseDefault: true}, "", false)
	if errMsg != "" || got == nil || !got.UseDefault {
		t.Fatalf("existing default: got=%+v err=%q", got, errMsg)
	}

	got, errMsg = buildEventReminders(req, nil, "", false)
	if errMsg != "" || got == nil || !got.UseDefault {
		t.Fatalf("fallback default: got=%+v err=%q", got, errMsg)
	}

	got, errMsg = buildEventReminders(req, nil, `[{"method":"email","minutes":10}]`, false)
	if errMsg != "" || got == nil || got.UseDefault || len(got.Overrides) != 1 {
		t.Fatalf("overrides: got=%+v err=%q", got, errMsg)
	}

	got, errMsg = buildEventReminders(req, nil, `{bad`, false)
	if errMsg == "" || got != nil {
		t.Fatalf("expected parse error, got=%+v err=%q", got, errMsg)
	}
}

func TestFormatGoogleMeetUpdate(t *testing.T) {
	t.Parallel()
	if got := formatGoogleMeetUpdate(false, nil); got != " (Google Meet removed)" {
		t.Fatalf("remove: %q", got)
	}
	if got := formatGoogleMeetUpdate(true, nil); got != "" {
		t.Fatalf("nil conference: %q", got)
	}
	got := formatGoogleMeetUpdate(true, &calendar.ConferenceData{
		EntryPoints: []*calendar.EntryPoint{
			{EntryPointType: "phone", Uri: "tel:1"},
			{EntryPointType: "video", Uri: "https://meet.google.com/abc"},
		},
	})
	if got != " Google Meet: https://meet.google.com/abc" {
		t.Fatalf("video: %q", got)
	}
	if got := formatGoogleMeetUpdate(true, &calendar.ConferenceData{
		EntryPoints: []*calendar.EntryPoint{{EntryPointType: "phone", Uri: "tel:1"}},
	}); got != "" {
		t.Fatalf("no video: %q", got)
	}
}
