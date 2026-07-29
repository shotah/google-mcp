package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	calendar "google.golang.org/api/calendar/v3"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// RegisterCalendarTools registers all Calendar tools with the MCP server.
func RegisterCalendarTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools
	registerListCalendars(s, getClient)
	registerGetEvents(s, getClient)
	registerQueryFreebusy(s, getClient)

	// Write tools
	registerCreateEvent(s, getClient)
	registerModifyEvent(s, getClient)
	registerDeleteEvent(s, getClient)
}

// newCalendarService creates a calendar.Service for the given user email.
func newCalendarService(ctx context.Context, getClient httpClientFunc, email string) (*calendar.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := calendar.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Calendar service: %w", err)
	}
	return svc, nil
}

// correctTimeFormatForAPI converts bare dates (YYYY-MM-DD) to RFC3339 and
// ensures timestamps end with a timezone designator.
func correctTimeFormatForAPI(t string) string {
	if t == "" {
		return ""
	}
	// Bare date: YYYY-MM-DD → append T00:00:00Z
	if len(t) == 10 && !strings.Contains(t, "T") {
		return t + "T00:00:00Z"
	}
	// Has time component but no timezone indicator
	if strings.Contains(t, "T") && !strings.Contains(t, "Z") && !strings.Contains(t, "+") && !strings.ContainsAny(t[len(t)-6:], "+-") {
		return t + "Z"
	}
	return t
}

// isAllDay returns true if the time string is a bare date (no "T" component).
func isAllDay(t string) bool {
	return !strings.Contains(t, "T")
}

// --- calendar_list_calendars ---

func registerListCalendars(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_list_calendars",
		mcp.WithDescription("List calendars the user can access (id + name). Use only when you need a non-primary calendar_id. For everyday primary calendar work, skip this and use calendar_id=\"primary\" with calendar_list_events / calendar_create_event / calendar_update_event."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
	)
	s.AddTool(tool, handleListCalendars(getClient))
}

func handleListCalendars(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.CalendarList.List().Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing calendars: %v", err)), nil
		}

		items := resp.Items
		if len(items) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No calendars found for %s.", email)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully listed %d calendars for %s:", len(items), email)
		for _, cal := range items {
			summary := cal.Summary
			if summary == "" {
				summary = "No Summary"
			}
			primary := ""
			if cal.Primary {
				primary = " (Primary)"
			}
			fmt.Fprintf(&b, "\n- \"%s\"%s (ID: %s)", summary, primary, cal.Id)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- calendar_list_events ---

func registerGetEvents(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_list_events",
		mcp.WithDescription("READ calendar: list events in a time range, or fetch one by event_id. Use for 'what's on my calendar', today/tomorrow, find an event before modify/delete. Day queries: user_google_email + calendar_id=\"primary\" + BOTH time_min and time_max (RFC3339) — do NOT set event_id. Only set event_id when you already have a real Google event id from a prior result. Never put \"primary\", a date, or a time range in event_id. Not for creating events — use calendar_create_event."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("calendar_id", mcp.Description("The ID of the calendar to query. Use 'primary' for the user's primary calendar. Defaults to 'primary'. Calendar IDs can be obtained using `calendar_list_calendars`.")),
		mcp.WithString("event_id", mcp.Description("Optional. Opaque Google Calendar event id from a previous result. If set, fetches that one event and ignores time_min/time_max. Do not use for day listings — never put 'primary', a date, or a time range here.")),
		mcp.WithString("time_min", mcp.Description("The start of the time range (inclusive) in RFC3339 format (e.g., '2026-07-28T00:00:00-07:00' or '2026-07-28'). If omitted, defaults to the current time. Ignored if event_id is provided.")),
		mcp.WithString("time_max", mcp.Description("The end of the time range (exclusive) in RFC3339. Required for clean day queries. If omitted, defaults to 24 hours after time_min (avoids unbounded future birthday spam). Ignored if event_id is provided.")),
		mcp.WithNumber("max_results", mcp.Description("The maximum number of events to return. Defaults to 25. Ignored if event_id is provided.")),
		mcp.WithString("query", mcp.Description("A keyword to search for within event fields (summary, description, location). Ignored if event_id is provided.")),
		mcp.WithBoolean("detailed", mcp.Description("Whether to return detailed event information including description, location, attendees, and attendee details (response status, organizer, optional flags). Defaults to False.")),
		mcp.WithBoolean("include_attachments", mcp.Description("Whether to include attachment information in detailed event output. When True, shows attachment details (fileId, fileUrl, mimeType, title) for events that have attachments. Only applies when detailed=True. Defaults to False.")),
	)
	s.AddTool(tool, handleGetEvents(getClient))
}

func handleGetEvents(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		calendarID := request.GetString("calendar_id", "primary")
		eventID := request.GetString("event_id", "")
		timeMin := request.GetString("time_min", "")
		timeMax := request.GetString("time_max", "")
		maxResults := request.GetInt("max_results", 25)
		query := request.GetString("query", "")
		detailed := getBool(request, "detailed", false)
		includeAttachments := getBool(request, "include_attachments", false)
		calendarID, eventID = normalizeGetEventsIDs(calendarID, eventID)

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, defaultedMax, err := getCalendarEventsWindow(svc, calendarID, eventID, timeMin, timeMax, maxResults, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(items) == 0 {
			if eventID != "" {
				return mcp.NewToolResultText(fmt.Sprintf("Event with ID '%s' not found in calendar '%s' for %s.", eventID, calendarID, email)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("No events found in calendar '%s' for %s for the specified time range.", calendarID, email)), nil
		}

		// Single event with detailed output
		if eventID != "" && detailed {
			item := items[0]
			return mcp.NewToolResultText(formatDetailedSingleEvent(item, eventID, includeAttachments)), nil
		}

		// Multiple events or single event with basic output
		var b strings.Builder
		if eventID != "" {
			fmt.Fprintf(&b, "Successfully retrieved event from calendar '%s' for %s:", calendarID, email)
		} else {
			fmt.Fprintf(&b, "Successfully retrieved %d events from calendar '%s' for %s:", len(items), calendarID, email)
			if defaultedMax {
				_, effMax, _ := resolveListWindow(timeMin, timeMax)
				fmt.Fprintf(&b, "\n(Note: time_max was omitted — used a 24h window ending %s. Pass an explicit time_max for the day next time.)", effMax)
			}
		}

		for _, item := range items {
			summary := item.Summary
			if summary == "" {
				summary = "No Title"
			}
			startTime := eventTime(item.Start)
			endTime := eventTime(item.End)
			link := item.HtmlLink
			if link == "" {
				link = "No Link"
			}
			itemEventID := item.Id
			if itemEventID == "" {
				itemEventID = "No ID"
			}

			if detailed {
				description := item.Description
				if description == "" {
					description = "No Description"
				}
				location := item.Location
				if location == "" {
					location = "No Location"
				}
				attendeeEmails := formatAttendeeEmails(item.Attendees)
				attendeeDetails := formatAttendeeDetails(item.Attendees, "    ")

				fmt.Fprintf(&b, "\n- \"%s\" (Starts: %s, Ends: %s)\n  Description: %s\n  Location: %s\n  Attendees: %s\n  Attendee Details: %s",
					summary, startTime, endTime, description, location, attendeeEmails, attendeeDetails)

				if includeAttachments {
					attachmentDetails := formatEventAttachmentDetails(item.Attachments, "    ")
					fmt.Fprintf(&b, "\n  Attachments: %s", attachmentDetails)
				}

				fmt.Fprintf(&b, "\n  ID: %s | Link: %s", itemEventID, link)
			} else {
				fmt.Fprintf(&b, "\n- \"%s\" (Starts: %s, Ends: %s) ID: %s | Link: %s",
					summary, startTime, endTime, itemEventID, link)
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- calendar_create_event ---

func registerCreateEvent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_create_event",
		mcp.WithDescription("CREATE a brand-new calendar event. Use for 'add/schedule/put on my calendar' when nothing exists yet. Required: user_google_email, summary, start_time, end_time (RFC3339). Optional location/description/attendees. Call immediately — do not narrate. NOT calendar_update_event (that needs event_id from calendar_list_events). If calendar_update_event errors with event_id required, call this instead."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Event title.")),
		mcp.WithString("start_time", mcp.Required(), mcp.Description("Start time (RFC3339, e.g., \"2023-10-27T10:00:00-07:00\" or \"2023-10-27\" for all-day).")),
		mcp.WithString("end_time", mcp.Required(), mcp.Description("End time (RFC3339, e.g., \"2023-10-27T11:00:00-07:00\" or \"2023-10-28\" for all-day).")),
		mcp.WithString("calendar_id", mcp.Description("Calendar ID (default: 'primary').")),
		mcp.WithString("description", mcp.Description("Event description.")),
		mcp.WithString("location", mcp.Description("Event location.")),
		mcp.WithArray("attendees", mcp.Description("Attendee email addresses."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithString("timezone", mcp.Description("Timezone (e.g., \"America/New_York\").")),
		mcp.WithArray("attachments", mcp.Description("List of Google Drive file URLs or IDs to attach to the event."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithBoolean("add_google_meet", mcp.Description("Whether to add a Google Meet video conference to the event. Defaults to False.")),
		mcp.WithString("reminders", mcp.Description("JSON string of reminder objects. Each should have 'method' (\"popup\" or \"email\") and 'minutes' (0-40320). Max 5 reminders. Example: '[{\"method\": \"popup\", \"minutes\": 15}]'")),
		mcp.WithBoolean("use_default_reminders", mcp.Description("Whether to use calendar's default reminders. If False, uses custom reminders. Defaults to True.")),
		mcp.WithString("transparency", mcp.Description("Event transparency for busy/free status. \"opaque\" shows as Busy (default), \"transparent\" shows as Available/Free."), mcp.Enum("opaque", "transparent")),
		mcp.WithString("visibility", mcp.Description("Event visibility. \"default\" uses calendar default, \"public\" is visible to all, \"private\" is visible only to attendees, \"confidential\" is same as private (legacy)."), mcp.Enum("default", "public", "private", "confidential")),
		mcp.WithBoolean("guests_can_modify", mcp.Description("Whether attendees other than the organizer can modify the event.")),
		mcp.WithBoolean("guests_can_invite_others", mcp.Description("Whether attendees other than the organizer can invite others to the event.")),
		mcp.WithBoolean("guests_can_see_other_guests", mcp.Description("Whether attendees other than the organizer can see who the event's attendees are.")),
	)
	s.AddTool(tool, handleCreateEvent(getClient))
}

func handleCreateEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		summary, err := request.RequireString("summary")
		if err != nil {
			return mcp.NewToolResultError("summary is required"), nil
		}
		startTime, err := request.RequireString("start_time")
		if err != nil {
			return mcp.NewToolResultError("start_time is required"), nil
		}
		endTime, err := request.RequireString("end_time")
		if err != nil {
			return mcp.NewToolResultError("end_time is required"), nil
		}

		calendarID := request.GetString("calendar_id", "")
		if calendarID == "" {
			calendarID = "primary"
		}
		description := request.GetString("description", "")
		location := request.GetString("location", "")
		attendees := getStringSlice(request, "attendees")
		timezone := request.GetString("timezone", "")
		attachmentIDs := getStringSlice(request, "attachments")
		addGoogleMeet := getBool(request, "add_google_meet", false)
		remindersStr := request.GetString("reminders", "")
		useDefaultReminders := getBool(request, "use_default_reminders", true)
		transparency := request.GetString("transparency", "")
		visibility := request.GetString("visibility", "")
		args := request.GetArguments()

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Build event body
		eventBody := &calendar.Event{
			Summary: summary,
		}

		// Set start/end times
		if isAllDay(startTime) {
			eventBody.Start = &calendar.EventDateTime{Date: startTime}
		} else {
			eventBody.Start = &calendar.EventDateTime{DateTime: startTime}
		}
		if isAllDay(endTime) {
			eventBody.End = &calendar.EventDateTime{Date: endTime}
		} else {
			eventBody.End = &calendar.EventDateTime{DateTime: endTime}
		}

		// Apply timezone if set
		if timezone != "" {
			if eventBody.Start.DateTime != "" {
				eventBody.Start.TimeZone = timezone
			}
			if eventBody.End.DateTime != "" {
				eventBody.End.TimeZone = timezone
			}
		}

		if description != "" {
			eventBody.Description = description
		}
		if location != "" {
			eventBody.Location = location
		}
		if len(attendees) > 0 {
			for _, a := range attendees {
				eventBody.Attendees = append(eventBody.Attendees, &calendar.EventAttendee{Email: a})
			}
		}

		// Handle reminders
		if remindersStr != "" || !useDefaultReminders {
			effectiveUseDefault := useDefaultReminders && remindersStr == ""
			reminderData := &calendar.EventReminders{
				UseDefault:      effectiveUseDefault,
				ForceSendFields: []string{"UseDefault"},
			}
			if remindersStr != "" {
				overrides, errMsg := parseRemindersJSON(remindersStr)
				if errMsg != "" {
					return mcp.NewToolResultError(errMsg), nil
				}
				reminderData.Overrides = overrides
				reminderData.UseDefault = false
			}
			eventBody.Reminders = reminderData
		}

		// Handle transparency
		if transparency != "" {
			eventBody.Transparency = transparency
		}

		// Handle visibility
		if visibility != "" {
			eventBody.Visibility = visibility
		}

		// Handle guest permissions
		if _, ok := args["guests_can_modify"]; ok {
			v := getBool(request, "guests_can_modify", false)
			eventBody.GuestsCanModify = v
			eventBody.ForceSendFields = append(eventBody.ForceSendFields, "GuestsCanModify")
		}
		if _, ok := args["guests_can_invite_others"]; ok {
			v := getBool(request, "guests_can_invite_others", true)
			eventBody.GuestsCanInviteOthers = &v
		}
		if _, ok := args["guests_can_see_other_guests"]; ok {
			v := getBool(request, "guests_can_see_other_guests", true)
			eventBody.GuestsCanSeeOtherGuests = &v
		}

		// Handle Google Meet
		conferenceDataVersion := int64(0)
		if addGoogleMeet {
			eventBody.ConferenceData = &calendar.ConferenceData{
				CreateRequest: &calendar.CreateConferenceRequest{
					RequestId: fmt.Sprintf("meet-%d", time.Now().UnixNano()),
					ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
						Type: "hangoutsMeet",
					},
				},
			}
			conferenceDataVersion = 1
		}

		// Handle attachments
		if len(attachmentIDs) > 0 {
			for _, att := range attachmentIDs {
				fileID := extractDriveFileID(att)
				if fileID != "" {
					eventBody.Attachments = append(eventBody.Attachments, &calendar.EventAttachment{
						FileUrl: "https://drive.google.com/open?id=" + fileID,
						Title:   "Drive Attachment",
					})
				}
			}
		}

		call := svc.Events.Insert(calendarID, eventBody).
			ConferenceDataVersion(conferenceDataVersion)
		if len(attachmentIDs) > 0 {
			call = call.SupportsAttachments(true)
		}

		created, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating event: %v", err)), nil
		}

		link := created.HtmlLink
		if link == "" {
			link = "No link available"
		}
		msg := fmt.Sprintf("Successfully created event '%s' for %s. Link: %s", created.Summary, email, link)

		// Add Meet link if created
		if addGoogleMeet && created.ConferenceData != nil {
			var msgSb456 strings.Builder
			for _, ep := range created.ConferenceData.EntryPoints {
				if ep.EntryPointType == "video" && ep.Uri != "" {
					fmt.Fprintf(&msgSb456, " Google Meet: %s", ep.Uri)
					break
				}
			}
			msg += msgSb456.String()
		}

		return mcp.NewToolResultText(msg), nil
	}
}

// --- calendar_update_event ---

func registerModifyEvent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_update_event",
		mcp.WithDescription("UPDATE an EXISTING calendar event (location, title, times, description). Required: user_google_email + event_id from calendar_list_events (ID: field). Use for 'change my 3pm', 'add address to that event'. After search finds an address for a named event, MUST call this with location before replying. NOT for new events — use calendar_create_event. If you lack event_id, calendar_list_events first (or calendar_create_event if new)."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("event_id", mcp.Required(), mcp.Description("Opaque Google event id from calendar_list_events (ID: …). Required for updates.")),
		mcp.WithString("calendar_id", mcp.Description("Calendar ID (default: 'primary').")),
		mcp.WithString("summary", mcp.Description("New event title.")),
		mcp.WithString("start_time", mcp.Description("New start time (RFC3339, e.g., \"2023-10-27T10:00:00-07:00\" or \"2023-10-27\" for all-day).")),
		mcp.WithString("end_time", mcp.Description("New end time (RFC3339, e.g., \"2023-10-27T11:00:00-07:00\" or \"2023-10-28\" for all-day).")),
		mcp.WithString("description", mcp.Description("New event description.")),
		mcp.WithString("location", mcp.Description("New event location / street address. Use this to write a searched address onto the event.")),
		mcp.WithArray("attendees", mcp.Description("Attendees as email strings. Supports: [\"email@example.com\"]. New attendees default to responseStatus=\"needsAction\"."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithString("timezone", mcp.Description("New timezone (e.g., \"America/New_York\").")),
		mcp.WithBoolean("add_google_meet", mcp.Description("Whether to add or remove Google Meet video conference. If True, adds Google Meet; if False, removes it; if not provided, leaves unchanged.")),
		mcp.WithString("reminders", mcp.Description("JSON string of reminder objects to replace existing reminders. Each should have 'method' (\"popup\" or \"email\") and 'minutes' (0-40320). Max 5 reminders.")),
		mcp.WithBoolean("use_default_reminders", mcp.Description("Whether to use calendar's default reminders. If specified, overrides current reminder settings.")),
		mcp.WithString("transparency", mcp.Description("Event transparency for busy/free status. \"opaque\" shows as Busy, \"transparent\" shows as Available/Free."), mcp.Enum("opaque", "transparent")),
		mcp.WithString("visibility", mcp.Description("Event visibility. \"default\" uses calendar default, \"public\" is visible to all, \"private\" is visible only to attendees, \"confidential\" is same as private (legacy)."), mcp.Enum("default", "public", "private", "confidential")),
		mcp.WithString("color_id", mcp.Description("Event color ID (1-11).")),
		mcp.WithBoolean("guests_can_modify", mcp.Description("Whether attendees other than the organizer can modify the event.")),
		mcp.WithBoolean("guests_can_invite_others", mcp.Description("Whether attendees other than the organizer can invite others to the event.")),
		mcp.WithBoolean("guests_can_see_other_guests", mcp.Description("Whether attendees other than the organizer can see who the event's attendees are.")),
	)
	s.AddTool(tool, handleModifyEvent(getClient))
}

func handleModifyEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("event_id is required"), nil
		}

		calendarID := request.GetString("calendar_id", "")
		if calendarID == "" {
			calendarID = "primary"
		}

		args := request.GetArguments()

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch existing event to preserve fields
		existing, err := svc.Events.Get(calendarID, eventID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("event not found: %v", err)), nil
		}

		// Start with existing event and modify provided fields
		eventBody := existing

		if v, ok := args["summary"]; ok {
			if s, ok := v.(string); ok {
				eventBody.Summary = s
			}
		}
		if v, ok := args["description"]; ok {
			if s, ok := v.(string); ok {
				eventBody.Description = s
			}
		}
		if v, ok := args["location"]; ok {
			if s, ok := v.(string); ok {
				eventBody.Location = s
			}
		}

		timezone := request.GetString("timezone", "")

		if s, ok := args["start_time"].(string); ok {
			eventBody.Start = eventDateTime(s, timezone)
		}
		if s, ok := args["end_time"].(string); ok {
			eventBody.End = eventDateTime(s, timezone)
		}

		// Handle attendees
		attendeeList := getStringSlice(request, "attendees")
		if attendeeList != nil {
			eventBody.Attendees = nil
			for _, a := range attendeeList {
				eventBody.Attendees = append(eventBody.Attendees, &calendar.EventAttendee{Email: a})
			}
		}

		// Handle color_id
		if v, ok := args["color_id"]; ok {
			if s, ok := v.(string); ok {
				eventBody.ColorId = s
			}
		}

		// Handle reminders
		remindersStr := request.GetString("reminders", "")
		_, useDefaultPresent := args["use_default_reminders"]
		if remindersStr != "" || useDefaultPresent {
			reminderData, errMsg := buildEventReminders(request, existing.Reminders, remindersStr, useDefaultPresent)
			if errMsg != "" {
				return mcp.NewToolResultError(errMsg), nil
			}
			eventBody.Reminders = reminderData
		}

		// Handle transparency
		if v, ok := args["transparency"]; ok {
			if s, ok := v.(string); ok {
				eventBody.Transparency = s
			}
		}

		// Handle visibility
		if v, ok := args["visibility"]; ok {
			if s, ok := v.(string); ok {
				eventBody.Visibility = s
			}
		}

		// Handle guest permissions
		if _, ok := args["guests_can_modify"]; ok {
			v := getBool(request, "guests_can_modify", false)
			eventBody.GuestsCanModify = v
			eventBody.ForceSendFields = append(eventBody.ForceSendFields, "GuestsCanModify")
		}
		if _, ok := args["guests_can_invite_others"]; ok {
			v := getBool(request, "guests_can_invite_others", true)
			eventBody.GuestsCanInviteOthers = &v
		}
		if _, ok := args["guests_can_see_other_guests"]; ok {
			v := getBool(request, "guests_can_see_other_guests", true)
			eventBody.GuestsCanSeeOtherGuests = &v
		}

		// Handle Google Meet
		if _, ok := args["add_google_meet"]; ok {
			addMeet := getBool(request, "add_google_meet", false)
			if addMeet {
				eventBody.ConferenceData = &calendar.ConferenceData{
					CreateRequest: &calendar.CreateConferenceRequest{
						RequestId: fmt.Sprintf("meet-%d", time.Now().UnixNano()),
						ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
							Type: "hangoutsMeet",
						},
					},
				}
			} else {
				// Remove Google Meet
				eventBody.ConferenceData = &calendar.ConferenceData{}
				eventBody.ForceSendFields = append(eventBody.ForceSendFields, "ConferenceData")
			}
		}

		updated, err := svc.Events.Update(calendarID, eventID, eventBody).
			ConferenceDataVersion(1).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("modifying event: %v", err)), nil
		}

		link := updated.HtmlLink
		if link == "" {
			link = "No link available"
		}
		msg := fmt.Sprintf("Successfully modified event '%s' (ID: %s) for %s. Link: %s",
			updated.Summary, eventID, email, link)

		// Add Meet link info
		if addMeet, ok := args["add_google_meet"].(bool); ok {
			msg += formatGoogleMeetUpdate(addMeet, updated.ConferenceData)
		}

		return mcp.NewToolResultText(msg), nil
	}
}

// normalizeGetEventsIDs clears model mistakes where calendar_id or a date/range
// is stuffed into event_id (which would force Events.Get and 404).
func normalizeGetEventsIDs(calendarID, eventID string) (string, string) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !bogusCalendarEventID(eventID) {
		if calendarID == "" {
			calendarID = "primary"
		}
		return calendarID, eventID
	}
	if strings.EqualFold(eventID, "primary") && strings.TrimSpace(calendarID) == "" {
		calendarID = "primary"
	}
	if strings.TrimSpace(calendarID) == "" {
		calendarID = "primary"
	}
	return calendarID, ""
}

func bogusCalendarEventID(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return false
	}
	switch lower {
	case "primary", "secondary":
		return true
	}
	if strings.Contains(lower, " to ") || strings.ContainsAny(s, " \t\n") {
		return true
	}
	if len(lower) == 10 && lower[4] == '-' && lower[7] == '-' {
		ok := true
		for i, r := range lower {
			if i == 4 || i == 7 {
				continue
			}
			if r < '0' || r > '9' {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	if strings.Contains(lower, "t") && strings.ContainsAny(lower, ":-z+") {
		for _, r := range lower {
			if r >= '0' && r <= '9' {
				return true
			}
		}
	}
	return false
}

// resolveListWindow returns API time_min / time_max for Events.List.
// When time_max is omitted, defaults to 24h after time_min so small models
// that forget the upper bound do not pull months of future birthdays.
func resolveListWindow(timeMin, timeMax string) (effectiveMin, effectiveMax string, defaultedMax bool) {
	effectiveMin = correctTimeFormatForAPI(timeMin)
	if effectiveMin == "" {
		effectiveMin = time.Now().UTC().Format(time.RFC3339)
	}
	effectiveMax = correctTimeFormatForAPI(timeMax)
	if effectiveMax == "" {
		effectiveMax = defaultTimeMaxAfter(effectiveMin)
		defaultedMax = true
	}
	return effectiveMin, effectiveMax, defaultedMax
}

func defaultTimeMaxAfter(timeMinRFC3339 string) string {
	parsed, err := time.Parse(time.RFC3339, timeMinRFC3339)
	if err != nil {
		parsed, err = time.Parse("2006-01-02T15:04:05Z07:00", timeMinRFC3339)
	}
	if err != nil {
		return time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	}
	return parsed.Add(24 * time.Hour).Format(time.RFC3339)
}

func getCalendarEventsWindow(svc *calendar.Service, calendarID, eventID, timeMin, timeMax string, maxResults int, query string) ([]*calendar.Event, bool, error) {
	if eventID != "" {
		event, err := svc.Events.Get(calendarID, eventID).Do()
		if err != nil {
			return nil, false, fmt.Errorf("getting event %s: %w", eventID, err)
		}
		return []*calendar.Event{event}, false, nil
	}
	effectiveTimeMin, effectiveTimeMax, defaultedMax := resolveListWindow(timeMin, timeMax)
	call := svc.Events.List(calendarID).
		TimeMin(effectiveTimeMin).
		TimeMax(effectiveTimeMax).
		MaxResults(int64(maxResults)).
		SingleEvents(true).
		OrderBy("startTime")
	if query != "" {
		call = call.Q(query)
	}
	resp, err := call.Do()
	if err != nil {
		return nil, defaultedMax, fmt.Errorf("listing events: %w", err)
	}
	return resp.Items, defaultedMax, nil
}

func eventDateTime(value, timezone string) *calendar.EventDateTime {
	if isAllDay(value) {
		return &calendar.EventDateTime{Date: value}
	}
	return &calendar.EventDateTime{DateTime: value, TimeZone: timezone}
}

func buildEventReminders(request mcp.CallToolRequest, existing *calendar.EventReminders, reminders string, useDefaultPresent bool) (*calendar.EventReminders, string) {
	reminderData := &calendar.EventReminders{ForceSendFields: []string{"UseDefault"}}
	if useDefaultPresent {
		reminderData.UseDefault = getBool(request, "use_default_reminders", true)
	} else if existing != nil {
		reminderData.UseDefault = existing.UseDefault
	} else {
		reminderData.UseDefault = true
	}
	if reminders == "" {
		return reminderData, ""
	}
	overrides, errMsg := parseRemindersJSON(reminders)
	if errMsg != "" {
		return nil, errMsg
	}
	reminderData.Overrides = overrides
	reminderData.UseDefault = false
	return reminderData, ""
}

func formatGoogleMeetUpdate(addMeet bool, conferenceData *calendar.ConferenceData) string {
	if !addMeet {
		return " (Google Meet removed)"
	}
	if conferenceData == nil {
		return ""
	}
	for _, entryPoint := range conferenceData.EntryPoints {
		if entryPoint.EntryPointType == "video" && entryPoint.Uri != "" {
			return " Google Meet: " + entryPoint.Uri
		}
	}
	return ""
}

// --- calendar_delete_event ---

func registerDeleteEvent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_delete_event",
		mcp.WithDescription("DELETE an existing calendar event. Required: user_google_email + event_id from calendar_list_events. Use for 'cancel/remove that event'. Confirm destructive deletes when unclear. Not for creating or editing — use calendar_create_event / calendar_update_event."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("event_id", mcp.Required(), mcp.Description("The ID of the event to delete.")),
		mcp.WithString("calendar_id", mcp.Description("Calendar ID (default: 'primary').")),
	)
	s.AddTool(tool, handleDeleteEvent(getClient))
}

func handleDeleteEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("event_id is required"), nil
		}

		calendarID := request.GetString("calendar_id", "")
		if calendarID == "" {
			calendarID = "primary"
		}

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Verify event exists first
		_, err = svc.Events.Get(calendarID, eventID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Event not found. The event with ID '%s' could not be found in calendar '%s'. This may be due to incorrect ID format or the event no longer exists.", eventID, calendarID)), nil
		}

		err = svc.Events.Delete(calendarID, eventID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting event: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted event (ID: %s) from calendar '%s' for %s.", eventID, calendarID, email)), nil
	}
}

// --- calendar_query_freebusy ---

func registerQueryFreebusy(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("calendar_query_freebusy",
		mcp.WithDescription("Free/busy blocks for one or more calendars in a time range. Use for 'am I free Friday afternoon', scheduling across calendars. Prefer calendar_list_events when you need event titles/details."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("time_min", mcp.Required(), mcp.Description("The start of the interval for the query in RFC3339 format (e.g., '2024-05-12T10:00:00Z' or '2024-05-12').")),
		mcp.WithString("time_max", mcp.Required(), mcp.Description("The end of the interval for the query in RFC3339 format (e.g., '2024-05-12T18:00:00Z' or '2024-05-12').")),
		mcp.WithArray("calendar_ids", mcp.Description("List of calendar identifiers to query. If not provided, queries the primary calendar."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithNumber("group_expansion_max", mcp.Description("Maximum number of calendar identifiers to be provided for a single group. Maximum value is 100.")),
		mcp.WithNumber("calendar_expansion_max", mcp.Description("Maximum number of calendars for which FreeBusy information is to be provided. Maximum value is 50.")),
	)
	s.AddTool(tool, handleQueryFreebusy(getClient))
}

func handleQueryFreebusy(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		timeMin, err := request.RequireString("time_min")
		if err != nil {
			return mcp.NewToolResultError("time_min is required"), nil
		}
		timeMax, err := request.RequireString("time_max")
		if err != nil {
			return mcp.NewToolResultError("time_max is required"), nil
		}

		calendarIDs := getStringSlice(request, "calendar_ids")
		if len(calendarIDs) == 0 {
			calendarIDs = []string{"primary"}
		}
		groupExpansionMax := request.GetInt("group_expansion_max", 0)
		calendarExpansionMax := request.GetInt("calendar_expansion_max", 0)

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Format time parameters
		formattedTimeMin := correctTimeFormatForAPI(timeMin)
		formattedTimeMax := correctTimeFormatForAPI(timeMax)

		// Build request body
		items := make([]*calendar.FreeBusyRequestItem, 0, len(calendarIDs))
		for _, id := range calendarIDs {
			items = append(items, &calendar.FreeBusyRequestItem{Id: id})
		}

		reqBody := &calendar.FreeBusyRequest{
			TimeMin: formattedTimeMin,
			TimeMax: formattedTimeMax,
			Items:   items,
		}
		if groupExpansionMax > 0 {
			reqBody.GroupExpansionMax = int64(groupExpansionMax)
		}
		if calendarExpansionMax > 0 {
			reqBody.CalendarExpansionMax = int64(calendarExpansionMax)
		}

		resp, err := svc.Freebusy.Query(reqBody).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("querying free/busy: %v", err)), nil
		}

		calendars := resp.Calendars
		timeMinResult := resp.TimeMin
		if timeMinResult == "" {
			timeMinResult = formattedTimeMin
		}
		timeMaxResult := resp.TimeMax
		if timeMaxResult == "" {
			timeMaxResult = formattedTimeMax
		}

		if len(calendars) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No free/busy information found for the requested calendars for %s.", email)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Free/Busy information for %s:\nTime range: %s to %s\n", email, timeMinResult, timeMaxResult)

		for calID, calData := range calendars {
			fmt.Fprintf(&b, "\nCalendar: %s", calID)

			if len(calData.Errors) > 0 {
				b.WriteString("\n  Errors:")
				for _, e := range calData.Errors {
					fmt.Fprintf(&b, "\n    - %s: %s", e.Domain, e.Reason)
				}
				b.WriteString("\n")
				continue
			}

			if len(calData.Busy) == 0 {
				b.WriteString("\n  Status: Free (no busy periods)")
			} else {
				fmt.Fprintf(&b, "\n  Busy periods: %d", len(calData.Busy))
				for _, period := range calData.Busy {
					fmt.Fprintf(&b, "\n    - %s to %s", period.Start, period.End)
				}
			}
			b.WriteString("\n")
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- Helper functions ---

// eventTime extracts the display time from an EventDateTime.
func eventTime(edt *calendar.EventDateTime) string {
	if edt == nil {
		return "Unknown"
	}
	if edt.DateTime != "" {
		return edt.DateTime
	}
	if edt.Date != "" {
		return edt.Date
	}
	return "Unknown"
}

// formatAttendeeEmails formats attendee emails as a comma-separated string.
func formatAttendeeEmails(attendees []*calendar.EventAttendee) string {
	if len(attendees) == 0 {
		return "None"
	}
	emails := make([]string, 0, len(attendees))
	for _, a := range attendees {
		emails = append(emails, a.Email)
	}
	return strings.Join(emails, ", ")
}

// formatAttendeeDetails formats attendee details with response status and flags.
func formatAttendeeDetails(attendees []*calendar.EventAttendee, _ string) string {
	if len(attendees) == 0 {
		return "None"
	}
	var b strings.Builder
	for i, a := range attendees {
		if i > 0 {
			b.WriteString(", ")
		}
		status := a.ResponseStatus
		if status == "" {
			status = "needsAction"
		}
		detail := fmt.Sprintf("%s (%s)", a.Email, status)
		if a.Organizer {
			detail += " [organizer]"
		}
		if a.Optional {
			detail += " [optional]"
		}
		b.WriteString(detail)
	}
	return b.String()
}

// formatEventAttachmentDetails formats attachment details for calendar events.
func formatEventAttachmentDetails(attachments []*calendar.EventAttachment, _ string) string {
	if len(attachments) == 0 {
		return "None"
	}
	var b strings.Builder
	for i, att := range attachments {
		if i > 0 {
			b.WriteString(", ")
		}
		title := att.Title
		if title == "" {
			title = "Untitled"
		}
		fmt.Fprintf(&b, "%s (URL: %s, MIME: %s, FileID: %s)", title, att.FileUrl, att.MimeType, att.FileId)
	}
	return b.String()
}

// formatDetailedSingleEvent formats a single event with full details.
func formatDetailedSingleEvent(item *calendar.Event, eventID string, includeAttachments bool) string {
	summary := item.Summary
	if summary == "" {
		summary = "No Title"
	}
	startTime := eventTime(item.Start)
	endTime := eventTime(item.End)
	link := item.HtmlLink
	if link == "" {
		link = "No Link"
	}
	description := item.Description
	if description == "" {
		description = "No Description"
	}
	location := item.Location
	if location == "" {
		location = "No Location"
	}
	colorID := item.ColorId
	if colorID == "" {
		colorID = "None"
	}
	attendeeEmails := formatAttendeeEmails(item.Attendees)
	attendeeDetails := formatAttendeeDetails(item.Attendees, "  ")

	var b strings.Builder
	fmt.Fprintf(&b, "Event Details:\n- Title: %s\n- Starts: %s\n- Ends: %s\n- Description: %s\n- Location: %s\n- Color ID: %s\n- Attendees: %s\n- Attendee Details: %s\n",
		summary, startTime, endTime, description, location, colorID, attendeeEmails, attendeeDetails)

	if includeAttachments {
		attachmentDetails := formatEventAttachmentDetails(item.Attachments, "  ")
		fmt.Fprintf(&b, "- Attachments: %s\n", attachmentDetails)
	}

	fmt.Fprintf(&b, "- Event ID: %s\n- Link: %s", eventID, link)
	return b.String()
}

// parseRemindersJSON parses a JSON string of reminder objects into calendar.EventReminder slice.
func parseRemindersJSON(s string) ([]*calendar.EventReminder, string) {
	var raw []map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Sprintf("invalid reminders JSON: %v", err)
	}
	if len(raw) > 5 {
		return nil, "maximum 5 reminders allowed"
	}

	var result []*calendar.EventReminder
	for _, r := range raw {
		method, _ := r["method"].(string)
		if method != "popup" && method != "email" {
			return nil, fmt.Sprintf("invalid reminder method: %q (must be 'popup' or 'email')", method)
		}
		minutesRaw, ok := r["minutes"]
		if !ok {
			return nil, "reminder must have 'minutes' field"
		}
		var minutes int64
		switch v := minutesRaw.(type) {
		case float64:
			minutes = int64(v)
		case int:
			minutes = int64(v)
		default:
			return nil, fmt.Sprintf("invalid minutes value: %v", minutesRaw)
		}
		if minutes < 0 || minutes > 40320 {
			return nil, fmt.Sprintf("minutes must be between 0 and 40320, got %d", minutes)
		}
		result = append(result, &calendar.EventReminder{
			Method:  method,
			Minutes: minutes,
		})
	}
	return result, ""
}

// extractDriveFileID extracts a Drive file ID from a URL or returns the string as-is if it's already an ID.
func extractDriveFileID(input string) string {
	if strings.HasPrefix(input, "https://") {
		// Try to extract file ID from various Drive URL formats
		for _, pattern := range []string{"/d/", "/file/d/"} {
			_, after, ok := strings.Cut(input, pattern)
			if !ok {
				continue
			}
			if slashIdx := strings.IndexAny(after, "/?"); slashIdx >= 0 {
				return after[:slashIdx]
			}
			return after
		}
		// Try id= parameter
		if _, after, ok := strings.Cut(input, "id="); ok {
			before, _, _ := strings.Cut(after, "&")
			return before
		}
		return ""
	}
	return input
}
