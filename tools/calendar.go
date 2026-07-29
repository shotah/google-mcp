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
	registerListEvents(s, getClient)
	registerGetEvent(s, getClient)
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
	tool := newMCPTool("calendar_list_calendars",
		mcp.WithDescription("List calendar ids/names. Skip for everyday primary work — use calendar_id=\"primary\"."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
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

func registerListEvents(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("calendar_list_events",
		mcp.WithDescription("List events in a time range (title, times, id). Use for today/tomorrow / find before edit. Day query: time_min + time_max (RFC3339), calendar_id=\"primary\". One known id → calendar_get_event. New event → calendar_create_event."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("time_min", mcp.Description("Range start RFC3339 (e.g. 2026-07-28T00:00:00-07:00). Default: now.")),
		mcp.WithString("time_max", mcp.Description("Range end RFC3339. Default: time_min+24h. Pass both for a clean day query.")),
		mcp.WithString("calendar_id", mcp.Description("Calendar id. Default: primary.")),
		mcp.WithString("query", mcp.Description("Optional keyword filter (title/description/location).")),
		mcp.WithNumber("max_results", mcp.Description("Max events. Default: 25.")),
	)
	s.AddTool(tool, handleListEvents(getClient))
}

func handleListEvents(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `calendar_list_events(time_min, time_max, calendar_id="primary")`), nil
		}

		calendarID := request.GetString("calendar_id", "primary")
		if strings.TrimSpace(calendarID) == "" {
			calendarID = "primary"
		}
		timeMin := request.GetString("time_min", "")
		timeMax := request.GetString("time_max", "")
		maxResults := request.GetInt("max_results", 25)
		query := request.GetString("query", "")

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		items, defaultedMax, err := listCalendarEvents(svc, calendarID, timeMin, timeMax, maxResults, query)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(items) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No events found in calendar '%s' for %s for the specified time range.", calendarID, email)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully retrieved %d events from calendar '%s' for %s:", len(items), calendarID, email)
		if defaultedMax {
			_, effMax, _ := resolveListWindow(timeMin, timeMax)
			fmt.Fprintf(&b, "\n(Note: time_max was omitted — used a 24h window ending %s. Next: pass time_max for the day.)", effMax)
		}

		for _, item := range items {
			summary := item.Summary
			if summary == "" {
				summary = "No Title"
			}
			link := item.HtmlLink
			if link == "" {
				link = "No Link"
			}
			itemEventID := item.Id
			if itemEventID == "" {
				itemEventID = "No ID"
			}
			fmt.Fprintf(&b, "\n- \"%s\" (Starts: %s, Ends: %s) ID: %s | Link: %s",
				summary, eventTime(item.Start), eventTime(item.End), itemEventID, link)
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- calendar_get_event ---

func registerGetEvent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("calendar_get_event",
		mcp.WithDescription("Get one event by event_id (details). Required: event_id from calendar_list_events. Day/range listing → calendar_list_events(time_min, time_max). Never event_id=\"primary\" or a date."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("event_id", mcp.Required(), mcp.Description("Opaque event id from calendar_list_events (ID: …). Not 'primary' or a date.")),
		mcp.WithString("calendar_id", mcp.Description("Calendar id. Default: primary.")),
	)
	s.AddTool(tool, handleGetEvent(getClient))
}

func handleGetEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "calendar_get_event(event_id=…)"), nil
		}
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return needArg("event_id", `calendar_get_event(event_id=…) or calendar_list_events(time_min, time_max)`), nil
		}
		eventID = strings.TrimSpace(eventID)
		if eventID == "" || bogusCalendarEventID(eventID) {
			return toolHint("event_id looks like a calendar id or date/range", `calendar_list_events(time_min, time_max, calendar_id="primary")`), nil
		}

		calendarID := request.GetString("calendar_id", "primary")
		if strings.TrimSpace(calendarID) == "" {
			calendarID = "primary"
		}

		svc, err := newCalendarService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		item, err := svc.Events.Get(calendarID, eventID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting event %s: %v", eventID, err)), nil
		}
		if item == nil {
			return mcp.NewToolResultText(fmt.Sprintf("Event with ID '%s' not found in calendar '%s' for %s.", eventID, calendarID, email)), nil
		}

		return mcp.NewToolResultText(formatDetailedSingleEvent(item, eventID, true)), nil
	}
}

// --- calendar_create_event ---

func registerCreateEvent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("calendar_create_event",
		mcp.WithDescription("Create a new calendar event. Required: summary, start_time, end_time (RFC3339). Existing event → calendar_update_event(event_id)."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Event title.")),
		mcp.WithString("start_time", mcp.Required(), mcp.Description("Start RFC3339 or YYYY-MM-DD (all-day).")),
		mcp.WithString("end_time", mcp.Required(), mcp.Description("End RFC3339 or YYYY-MM-DD (all-day).")),
		mcp.WithString("calendar_id", mcp.Description("Calendar id. Default: primary.")),
		mcp.WithString("description", mcp.Description("Event description.")),
		mcp.WithString("location", mcp.Description("Event location / address.")),
		mcp.WithArray("attendees", mcp.Description("Attendee emails."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithString("timezone", mcp.Description("Timezone (e.g. America/Los_Angeles).")),
		mcp.WithBoolean("add_google_meet", mcp.Description("Add Google Meet. Default: false.")),
	)
	s.AddTool(tool, handleCreateEvent(getClient))
}

func handleCreateEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "calendar_create_event(summary, start_time, end_time)"), nil
		}
		summary, err := request.RequireString("summary")
		if err != nil {
			return needArg("summary", "calendar_create_event(summary, start_time, end_time)"), nil
		}
		startTime, err := request.RequireString("start_time")
		if err != nil {
			return needArg("start_time", "calendar_create_event(summary, start_time, end_time)"), nil
		}
		endTime, err := request.RequireString("end_time")
		if err != nil {
			return needArg("end_time", "calendar_create_event(summary, start_time, end_time)"), nil
		}

		calendarID := request.GetString("calendar_id", "")
		if calendarID == "" {
			calendarID = "primary"
		}
		description := request.GetString("description", "")
		location := request.GetString("location", "")
		attendees := getStringSlice(request, "attendees")
		timezone := request.GetString("timezone", "")
		// Advanced fields still accepted if a client sends them (not in lean schema).
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
	tool := newMCPTool("calendar_update_event",
		mcp.WithDescription("Update an existing event. Required: event_id from calendar_list_events. New event → calendar_create_event. Missing id → calendar_list_events first."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("event_id", mcp.Required(), mcp.Description("Opaque event id from calendar_list_events (ID: …).")),
		mcp.WithString("calendar_id", mcp.Description("Calendar id. Default: primary.")),
		mcp.WithString("summary", mcp.Description("New title.")),
		mcp.WithString("start_time", mcp.Description("New start RFC3339 or YYYY-MM-DD.")),
		mcp.WithString("end_time", mcp.Description("New end RFC3339 or YYYY-MM-DD.")),
		mcp.WithString("description", mcp.Description("New description.")),
		mcp.WithString("location", mcp.Description("New location / address.")),
		mcp.WithArray("attendees", mcp.Description("Attendee emails."), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithString("timezone", mcp.Description("Timezone (e.g. America/Los_Angeles).")),
		mcp.WithBoolean("add_google_meet", mcp.Description("true=add Meet, false=remove, omit=unchanged.")),
	)
	s.AddTool(tool, handleModifyEvent(getClient))
}

func handleModifyEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "calendar_update_event(event_id, …)"), nil
		}
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return needArg("event_id", `calendar_list_events(time_min, time_max) then calendar_update_event(event_id, …)`), nil
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
			return toolHint(fmt.Sprintf("event not found: %v", err), `calendar_list_events(time_min, time_max) then calendar_update_event(event_id, …)`), nil
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

// bogusCalendarEventID reports values that models mistake for event ids
// (calendar_id "primary", dates, time ranges). calendar_get_event rejects these.
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

func listCalendarEvents(svc *calendar.Service, calendarID, timeMin, timeMax string, maxResults int, query string) ([]*calendar.Event, bool, error) {
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
	tool := newMCPTool("calendar_delete_event",
		mcp.WithDescription("Delete an event. Required: event_id from calendar_list_events. Confirm when unclear."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("event_id", mcp.Required(), mcp.Description("Opaque event id from calendar_list_events (ID: …).")),
		mcp.WithString("calendar_id", mcp.Description("Calendar id. Default: primary.")),
	)
	s.AddTool(tool, handleDeleteEvent(getClient))
}

func handleDeleteEvent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "calendar_delete_event(event_id=…)"), nil
		}
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return needArg("event_id", `calendar_list_events(time_min, time_max) then calendar_delete_event(event_id)`), nil
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
	tool := newMCPTool("calendar_query_freebusy",
		mcp.WithDescription("Free/busy blocks in a time range. Event titles → calendar_list_events."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("time_min", mcp.Required(), mcp.Description("Range start RFC3339.")),
		mcp.WithString("time_max", mcp.Required(), mcp.Description("Range end RFC3339.")),
		mcp.WithArray("calendar_ids", mcp.Description("Calendar ids. Default: primary."), mcp.Items(map[string]any{"type": "string"})),
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
