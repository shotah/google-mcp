package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/server"
)

// registeredToolNames returns the names of all tools registered on the server.
func registeredToolNames(t *testing.T, s *mcpserver.MCPServer) map[string]bool {
	t.Helper()
	ctx := context.Background()
	resp := s.HandleMessage(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	result, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	listResult, ok := result.Result.(mcp.ListToolsResult)
	if !ok {
		t.Fatalf("expected ListToolsResult, got %T", result.Result)
	}

	names := make(map[string]bool, len(listResult.Tools))
	for _, tool := range listResult.Tools {
		names[tool.Name] = true
	}
	return names
}

// newTestServer creates an MCP server and registers all tools, then applies filtering.
func newTestServer(t *testing.T, cfg server.Config) *mcpserver.MCPServer {
	t.Helper()
	s := server.New(cfg)
	RegisterAllTools(s, cfg)
	FilterTools(s, cfg)
	return s
}

func TestNoFilterLoadsAllTools(t *testing.T) {
	s := newTestServer(t, server.Config{})
	names := registeredToolNames(t, s)
	// 12 comment tools + 15 Gmail + 16 Drive + 7 Calendar + 15 Docs + 10 Sheets + 4 Chat + 6 Forms + 5 Slides + 12 Tasks + 15 Contacts + 3 Search + 17 AppScript + 1 auth_start = 138.
	if len(names) != 138 {
		t.Errorf("expected 138 tools with no filter, got %d: %v", len(names), names)
	}
}

func TestTierCoreFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{ToolTier: "core"})
	names := registeredToolNames(t, s)
	// Gmail core (5) + Drive core (7) + Calendar core (6, includes calendar_get_event + calendar_delete_event) + Docs core (3) + Sheets core (3) + Chat core (3) + Forms core (2) + Slides core (2) + Tasks core (4) + Contacts core (4) + Search core (1) + AppScript core (7) = 47.
	if len(names) != 47 {
		t.Errorf("expected 47 tools with core tier, got %d: %v", len(names), names)
	}
	if !names["calendar_delete_event"] {
		t.Error("expected calendar_delete_event in core tier")
	}
}

func TestTierExtendedFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{ToolTier: "extended"})
	names := registeredToolNames(t, s)
	// Gmail core+extended (13) + Drive core+extended (14) + Calendar core+extended (7) + Docs core+extended (9) + Sheets core+extended (5) + Chat core+extended (4) + Forms core+extended (3) + Slides core+extended (5) + Tasks core+extended (5) + Contacts core+extended (8) + Search core+extended (2) + AppScript core+extended (17) = 92.
	if len(names) != 92 {
		t.Errorf("expected 92 tools with extended tier, got %d: %v", len(names), names)
	}
}

func TestTierCompleteFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{ToolTier: "complete"})
	names := registeredToolNames(t, s)
	// Complete tier = all tools, same as no filter.
	if len(names) != 138 {
		t.Errorf("expected 138 tools with complete tier, got %d: %v", len(names), names)
	}
}

func TestReadOnlyFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{ReadOnly: true})
	names := registeredToolNames(t, s)
	// 3 read comment + 8 Gmail read + 7 Drive read + 4 Calendar read + 6 Docs read + 3 Sheets read + 3 Chat read + 3 Forms read + 3 Slides read + 4 Tasks read + 5 Contacts read + 3 Search read + 8 AppScript read = 60.
	if len(names) != 60 {
		t.Errorf("expected 60 tools in read-only mode, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"docs_read_comments",
		"sheets_read_comments",
		"slides_read_comments",
		"gmail_search_messages",
		"gmail_get_message",
		"gmail_get_messages_batch",
		"gmail_get_attachment",
		"gmail_get_thread",
		"gmail_get_threads_batch",
		"gmail_list_labels",
		"gmail_list_filters",
		"drive_search_files",
		"drive_get_file_content",
		"drive_get_file_download_url",
		"drive_list_items",
		"drive_get_file_permissions",
		"drive_check_file_public_access",
		"drive_get_shareable_link",
		"calendar_list_calendars",
		"calendar_list_events",
		"calendar_get_event",
		"calendar_query_freebusy",
		"docs_search",
		"docs_get_content",
		"docs_list_in_folder",
		"docs_inspect_structure",
		"docs_debug_table_structure",
		"docs_export_to_pdf",
		"sheets_list_spreadsheets",
		"sheets_get_spreadsheet_info",
		"sheets_read_values",
		"chat_list_spaces",
		"chat_list_messages",
		"chat_search_messages",
		"forms_get",
		"forms_get_response",
		"forms_list_responses",
		"slides_get_presentation",
		"slides_get_page",
		"slides_get_page_thumbnail",
		"tasks_get_task",
		"tasks_list_tasks",
		"tasks_list_tasklists",
		"tasks_get_tasklist",
		"contacts_search",
		"contacts_get",
		"contacts_list",
		"contacts_list_groups",
		"contacts_get_group",
		"search_query",
		"search_get_engine_info",
		"search_query_siterestrict",
		"appscript_list_projects",
		"appscript_get_project",
		"appscript_get_content",
		"appscript_list_deployments",
		"appscript_list_processes",
		"appscript_list_versions",
		"appscript_get_version",
		"appscript_get_metrics",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present in read-only mode", expected)
		}
	}
}

func TestReadOnlyPlusTierComposition(t *testing.T) {
	// Read-only + complete tier: all read-only tools survive.
	s := newTestServer(t, server.Config{ReadOnly: true, ToolTier: "complete"})
	names := registeredToolNames(t, s)
	if len(names) != 60 {
		t.Errorf("expected 60 tools with read-only + complete tier, got %d: %v", len(names), names)
	}

	// Read-only + core tier: Gmail core read-only (3) + Drive core read-only (4)
	// + Calendar core read-only (3) + Docs core read-only (1: docs_get_content) + Sheets core read-only (1: sheets_read_values)
	// + Chat core read-only (2: chat_list_messages, chat_search_messages) + Forms core read-only (1: forms_get)
	// + Slides core read-only (1: slides_get_presentation) + Tasks core read-only (2: tasks_get_task, tasks_list_tasks)
	// + Contacts core read-only (3: contacts_search, contacts_get, contacts_list) + Search core read-only (1: search_query)
	// + AppScript core read-only (3: appscript_list_projects, appscript_get_project, appscript_get_content) = 25.
	s2 := newTestServer(t, server.Config{ReadOnly: true, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 25 {
		t.Errorf("expected 25 tools with read-only + core tier, got %d: %v", len(names2), names2)
	}
}

func TestCapabilityReadFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{Capability: "read"})
	names := registeredToolNames(t, s)
	if len(names) != 60 {
		t.Errorf("expected 60 tools with capability read, got %d: %v", len(names), names)
	}
	if names["calendar_delete_event"] {
		t.Error("calendar_delete_event must not appear under capability read")
	}
	if names["calendar_create_event"] {
		t.Error("calendar_create_event must not appear under capability read")
	}
}

func TestCapabilityEditFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{Capability: "edit"})
	names := registeredToolNames(t, s)
	// All tools minus 6 destructive = 132.
	if len(names) != 132 {
		t.Errorf("expected 132 tools with capability edit, got %d: %v", len(names), names)
	}
	if !names["calendar_delete_event"] {
		t.Error("expected calendar_delete_event under capability edit")
	}
	if !names["calendar_create_event"] {
		t.Error("expected calendar_create_event under capability edit")
	}
	for _, destructive := range []string{
		"drive_transfer_ownership",
		"contacts_batch_delete",
		"tasks_delete_tasklist",
		"contacts_delete_group",
		"appscript_delete_project",
		"tasks_clear_completed",
	} {
		if names[destructive] {
			t.Errorf("%s must not appear under capability edit", destructive)
		}
	}
}

func TestCapabilityCompleteFiltering(t *testing.T) {
	s := newTestServer(t, server.Config{Capability: "complete"})
	names := registeredToolNames(t, s)
	if len(names) != 138 {
		t.Errorf("expected 138 tools with capability complete, got %d: %v", len(names), names)
	}
	if !names["drive_transfer_ownership"] {
		t.Error("expected drive_transfer_ownership under capability complete")
	}
}

func TestCapabilityEditPlusCore(t *testing.T) {
	s := newTestServer(t, server.Config{ToolTier: "core", Capability: "edit"})
	names := registeredToolNames(t, s)
	// Core has no destructive tools, so edit does not shrink core further.
	if len(names) != 47 {
		t.Errorf("expected 47 tools with core+edit, got %d: %v", len(names), names)
	}
	if !names["calendar_delete_event"] {
		t.Error("expected calendar_delete_event with core+edit")
	}
}

func TestReadOnlyOverridesCapability(t *testing.T) {
	// --read-only wins even if --capability edit is set.
	s := newTestServer(t, server.Config{Capability: "edit", ReadOnly: true})
	names := registeredToolNames(t, s)
	if len(names) != 60 {
		t.Errorf("expected 60 tools when read-only overrides edit, got %d: %v", len(names), names)
	}
	if names["calendar_delete_event"] {
		t.Error("calendar_delete_event must not appear when read-only overrides edit")
	}
}

func TestLeanPresetSurface(t *testing.T) {
	// --preset lean → gmail + calendar, core, edit ≈ 11 tools (auth_start is complete-tier only).
	s := newTestServer(t, server.Config{
		Tools:      []string{"gmail", "calendar"},
		ToolTier:   "core",
		Capability: "edit",
	})
	names := registeredToolNames(t, s)
	if len(names) != 11 {
		t.Errorf("expected 11 tools for lean preset, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"gmail_search_messages",
		"gmail_get_message",
		"gmail_get_messages_batch",
		"gmail_send_message",
		"gmail_modify_message_labels",
		"calendar_list_calendars",
		"calendar_list_events",
		"calendar_get_event",
		"calendar_create_event",
		"calendar_update_event",
		"calendar_delete_event",
	} {
		if !names[expected] {
			t.Errorf("expected lean tool %q", expected)
		}
	}
	if names["auth_start"] {
		t.Error("auth_start must not appear on lean (core) surface")
	}
	if names["drive_search_files"] {
		t.Error("drive tools must not appear on lean surface")
	}
}

func TestEverydayPresetSurface(t *testing.T) {
	// --preset everyday → gmail + calendar + docs + sheets + tasks, core, edit ≈ 21 tools.
	// Drive is not required for Docs/Sheets core work.
	s := newTestServer(t, server.Config{
		Tools:      []string{"gmail", "calendar", "docs", "sheets", "tasks"},
		ToolTier:   "core",
		Capability: "edit",
	})
	names := registeredToolNames(t, s)
	if len(names) != 21 {
		t.Errorf("expected 21 tools for everyday preset, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"gmail_modify_message_labels",
		"docs_get_content",
		"docs_create",
		"docs_modify_text",
		"sheets_create_spreadsheet",
		"sheets_read_values",
		"sheets_modify_values",
		"tasks_list_tasks",
		"tasks_get_task",
		"tasks_create_task",
		"tasks_update_task",
	} {
		if !names[expected] {
			t.Errorf("expected everyday tool %q", expected)
		}
	}
	if names["drive_search_files"] {
		t.Error("drive tools must not appear on everyday surface")
	}
	if names["sheets_list_spreadsheets"] {
		t.Error("sheets_list_spreadsheets is extended — not on everyday core")
	}
	if names["tasks_list_tasklists"] {
		t.Error("tasks_list_tasklists is complete — not on everyday core")
	}
}

func TestToolsFilterComposesWithServiceFilter(t *testing.T) {
	// --tools docs with no tier: 15 Docs tools + 4 comment tools = 19.
	s := newTestServer(t, server.Config{Tools: []string{"docs"}})
	names := registeredToolNames(t, s)
	if len(names) != 19 {
		t.Errorf("expected 19 tools with --tools docs, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"docs_read_comments",
		"docs_create_comment",
		"docs_reply_to_comment",
		"docs_resolve_comment",
		"docs_search",
		"docs_get_content",
		"docs_list_in_folder",
		"docs_create",
		"docs_inspect_structure",
		"docs_debug_table_structure",
		"docs_export_to_pdf",
		"docs_modify_text",
		"docs_find_and_replace",
		"docs_insert_elements",
		"docs_insert_image",
		"docs_update_headers_footers",
		"docs_batch_update",
		"docs_create_table_with_data",
		"docs_update_paragraph_style",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}
}

func TestToolsFilterPlusTier(t *testing.T) {
	// --tools docs --tool-tier core: docs_get_content + docs_create + docs_modify_text = 3.
	s := newTestServer(t, server.Config{Tools: []string{"docs"}, ToolTier: "core"})
	names := registeredToolNames(t, s)
	if len(names) != 3 {
		t.Errorf("expected 3 tools with --tools docs --tool-tier core, got %d: %v", len(names), names)
	}
}

func TestToolsFilterPlusReadOnly(t *testing.T) {
	// --tools docs --read-only: 6 Docs read-only + docs_read_comments = 7.
	s := newTestServer(t, server.Config{Tools: []string{"docs"}, ReadOnly: true})
	names := registeredToolNames(t, s)
	if len(names) != 7 {
		t.Errorf("expected 7 tools with --tools docs --read-only, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"docs_read_comments",
		"docs_search",
		"docs_get_content",
		"docs_list_in_folder",
		"docs_inspect_structure",
		"docs_debug_table_structure",
		"docs_export_to_pdf",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}
}

func TestToolsGmailFiltering(t *testing.T) {
	// --tools gmail: all 15 Gmail tools (7 read + 8 write) + 1 auth_start = 16.
	s := newTestServer(t, server.Config{Tools: []string{"gmail"}})
	names := registeredToolNames(t, s)
	if len(names) != 16 {
		t.Errorf("expected 16 tools with --tools gmail, got %d: %v", len(names), names)
	}

	// --tools gmail --tool-tier core: 5 core Gmail tools (includes label modify for trash/archive).
	s2 := newTestServer(t, server.Config{Tools: []string{"gmail"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 5 {
		t.Errorf("expected 5 tools with --tools gmail --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools gmail --read-only: 7 Gmail read tools + gmail_list_filters = 8.
	s3 := newTestServer(t, server.Config{Tools: []string{"gmail"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 8 {
		t.Errorf("expected 8 tools with --tools gmail --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsDriveFiltering(t *testing.T) {
	// --tools drive: 16 Drive tools (7 read + 9 write).
	s := newTestServer(t, server.Config{Tools: []string{"drive"}})
	names := registeredToolNames(t, s)
	if len(names) != 16 {
		t.Errorf("expected 16 tools with --tools drive, got %d: %v", len(names), names)
	}

	// --tools drive --tool-tier core: 7 Drive core tools.
	s2 := newTestServer(t, server.Config{Tools: []string{"drive"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 7 {
		t.Errorf("expected 7 tools with --tools drive --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools drive --read-only: 7 Drive read-only tools.
	s3 := newTestServer(t, server.Config{Tools: []string{"drive"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 7 {
		t.Errorf("expected 7 tools with --tools drive --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsCalendarFiltering(t *testing.T) {
	// --tools calendar: all 7 Calendar tools.
	s := newTestServer(t, server.Config{Tools: []string{"calendar"}})
	names := registeredToolNames(t, s)
	if len(names) != 7 {
		t.Errorf("expected 7 tools with --tools calendar, got %d: %v", len(names), names)
	}

	// --tools calendar --tool-tier core: 6 core Calendar tools (includes calendar_delete_event).
	s2 := newTestServer(t, server.Config{Tools: []string{"calendar"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 6 {
		t.Errorf("expected 6 tools with --tools calendar --tool-tier core, got %d: %v", len(names2), names2)
	}
	if !names2["calendar_delete_event"] {
		t.Error("expected calendar_delete_event with --tools calendar --tool-tier core")
	}

	// --tools calendar --read-only: 4 Calendar read-only tools.
	s3 := newTestServer(t, server.Config{Tools: []string{"calendar"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 4 {
		t.Errorf("expected 4 tools with --tools calendar --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsDocsFiltering(t *testing.T) {
	// --tools docs: 15 Docs tools + 4 comment tools = 19.
	s := newTestServer(t, server.Config{Tools: []string{"docs"}})
	names := registeredToolNames(t, s)
	if len(names) != 19 {
		t.Errorf("expected 19 tools with --tools docs, got %d: %v", len(names), names)
	}

	// --tools docs --tool-tier core: docs_get_content + docs_create + docs_modify_text = 3.
	s2 := newTestServer(t, server.Config{Tools: []string{"docs"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 3 {
		t.Errorf("expected 3 tools with --tools docs --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools docs --read-only: 6 Docs read-only + docs_read_comments = 7.
	s3 := newTestServer(t, server.Config{Tools: []string{"docs"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 7 {
		t.Errorf("expected 7 tools with --tools docs --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsSheetsFiltering(t *testing.T) {
	// --tools sheets: 10 Sheets tools + 4 comment tools = 14.
	s := newTestServer(t, server.Config{Tools: []string{"sheets"}})
	names := registeredToolNames(t, s)
	if len(names) != 14 {
		t.Errorf("expected 14 tools with --tools sheets, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"sheets_list_spreadsheets",
		"sheets_get_spreadsheet_info",
		"sheets_read_values",
		"sheets_modify_values",
		"sheets_format_range",
		"sheets_add_conditional_formatting",
		"sheets_update_conditional_formatting",
		"sheets_delete_conditional_formatting",
		"sheets_create_spreadsheet",
		"sheets_create_sheet",
		"sheets_read_comments",
		"sheets_create_comment",
		"sheets_reply_to_comment",
		"sheets_resolve_comment",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools sheets --tool-tier core: 3 core Sheets tools.
	s2 := newTestServer(t, server.Config{Tools: []string{"sheets"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 3 {
		t.Errorf("expected 3 tools with --tools sheets --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools sheets --read-only: 3 Sheets read-only + sheets_read_comments = 4.
	s3 := newTestServer(t, server.Config{Tools: []string{"sheets"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 4 {
		t.Errorf("expected 4 tools with --tools sheets --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsChatFiltering(t *testing.T) {
	// --tools chat: all 4 Chat tools.
	s := newTestServer(t, server.Config{Tools: []string{"chat"}})
	names := registeredToolNames(t, s)
	if len(names) != 4 {
		t.Errorf("expected 4 tools with --tools chat, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"chat_list_spaces",
		"chat_list_messages",
		"chat_send_message",
		"chat_search_messages",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools chat --tool-tier core: 3 core Chat tools (chat_send_message, chat_list_messages, chat_search_messages).
	s2 := newTestServer(t, server.Config{Tools: []string{"chat"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 3 {
		t.Errorf("expected 3 tools with --tools chat --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools chat --read-only: 3 Chat read-only tools (chat_list_spaces, chat_list_messages, chat_search_messages).
	s3 := newTestServer(t, server.Config{Tools: []string{"chat"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 3 {
		t.Errorf("expected 3 tools with --tools chat --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsFormsFiltering(t *testing.T) {
	// --tools forms: all 6 Forms tools.
	s := newTestServer(t, server.Config{Tools: []string{"forms"}})
	names := registeredToolNames(t, s)
	if len(names) != 6 {
		t.Errorf("expected 6 tools with --tools forms, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"forms_create",
		"forms_get",
		"forms_set_publish_settings",
		"forms_get_response",
		"forms_list_responses",
		"forms_batch_update",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools forms --tool-tier core: 2 core Forms tools (forms_create, forms_get).
	s2 := newTestServer(t, server.Config{Tools: []string{"forms"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 2 {
		t.Errorf("expected 2 tools with --tools forms --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools forms --read-only: 3 Forms read-only tools (forms_get, forms_get_response, forms_list_responses).
	s3 := newTestServer(t, server.Config{Tools: []string{"forms"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 3 {
		t.Errorf("expected 3 tools with --tools forms --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsSlidesFiltering(t *testing.T) {
	// --tools slides: 5 Slides tools + 4 comment tools = 9.
	s := newTestServer(t, server.Config{Tools: []string{"slides"}})
	names := registeredToolNames(t, s)
	if len(names) != 9 {
		t.Errorf("expected 9 tools with --tools slides, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"slides_create_presentation",
		"slides_get_presentation",
		"slides_batch_update",
		"slides_get_page",
		"slides_get_page_thumbnail",
		"slides_read_comments",
		"slides_create_comment",
		"slides_reply_to_comment",
		"slides_resolve_comment",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools slides --tool-tier core: 2 core Slides tools (slides_create_presentation, slides_get_presentation).
	s2 := newTestServer(t, server.Config{Tools: []string{"slides"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 2 {
		t.Errorf("expected 2 tools with --tools slides --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools slides --read-only: 3 Slides read-only (slides_get_presentation, slides_get_page, slides_get_page_thumbnail) + slides_read_comments = 4.
	s3 := newTestServer(t, server.Config{Tools: []string{"slides"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 4 {
		t.Errorf("expected 4 tools with --tools slides --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsTasksFiltering(t *testing.T) {
	// --tools tasks: all 12 Tasks tools.
	s := newTestServer(t, server.Config{Tools: []string{"tasks"}})
	names := registeredToolNames(t, s)
	if len(names) != 12 {
		t.Errorf("expected 12 tools with --tools tasks, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"tasks_list_tasklists",
		"tasks_get_tasklist",
		"tasks_create_tasklist",
		"tasks_update_tasklist",
		"tasks_delete_tasklist",
		"tasks_list_tasks",
		"tasks_get_task",
		"tasks_create_task",
		"tasks_update_task",
		"tasks_delete_task",
		"tasks_move_task",
		"tasks_clear_completed",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools tasks --tool-tier core: 4 core Tasks tools (tasks_get_task, tasks_list_tasks, tasks_create_task, tasks_update_task).
	s2 := newTestServer(t, server.Config{Tools: []string{"tasks"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 4 {
		t.Errorf("expected 4 tools with --tools tasks --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools tasks --read-only: 4 Tasks read-only tools (tasks_get_task, tasks_list_tasks, tasks_list_tasklists, tasks_get_tasklist).
	s3 := newTestServer(t, server.Config{Tools: []string{"tasks"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 4 {
		t.Errorf("expected 4 tools with --tools tasks --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsContactsFiltering(t *testing.T) {
	// --tools contacts: 5 read + 10 write = 15 Contacts tools.
	s := newTestServer(t, server.Config{Tools: []string{"contacts"}})
	names := registeredToolNames(t, s)
	if len(names) != 15 {
		t.Errorf("expected 15 tools with --tools contacts, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"contacts_list",
		"contacts_get",
		"contacts_search",
		"contacts_list_groups",
		"contacts_get_group",
		"contacts_create",
		"contacts_update",
		"contacts_delete",
		"contacts_batch_create",
		"contacts_batch_update",
		"contacts_batch_delete",
		"contacts_create_group",
		"contacts_update_group",
		"contacts_delete_group",
		"contacts_modify_group_members",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools contacts --tool-tier core: 4 core Contacts tools (contacts_search, contacts_get, contacts_list, contacts_create).
	s2 := newTestServer(t, server.Config{Tools: []string{"contacts"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 4 {
		t.Errorf("expected 4 tools with --tools contacts --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools contacts --read-only: 5 Contacts read-only tools (all read tools remain, write tools removed).
	s3 := newTestServer(t, server.Config{Tools: []string{"contacts"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 5 {
		t.Errorf("expected 5 tools with --tools contacts --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsSearchFiltering(t *testing.T) {
	// --tools search: 3 Search tools.
	s := newTestServer(t, server.Config{Tools: []string{"search"}})
	names := registeredToolNames(t, s)
	if len(names) != 3 {
		t.Errorf("expected 3 tools with --tools search, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"search_query",
		"search_get_engine_info",
		"search_query_siterestrict",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools search --tool-tier core: 1 core Search tool (search_query).
	s2 := newTestServer(t, server.Config{Tools: []string{"search"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 1 {
		t.Errorf("expected 1 tool with --tools search --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools search --read-only: all 3 Search tools (all read-only).
	s3 := newTestServer(t, server.Config{Tools: []string{"search"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 3 {
		t.Errorf("expected 3 tools with --tools search --read-only, got %d: %v", len(names3), names3)
	}
}

func TestToolsAppScriptFiltering(t *testing.T) {
	// --tools appscript: 8 read + 9 write = 17 AppScript tools.
	s := newTestServer(t, server.Config{Tools: []string{"appscript"}})
	names := registeredToolNames(t, s)
	if len(names) != 17 {
		t.Errorf("expected 17 tools with --tools appscript, got %d: %v", len(names), names)
	}
	for _, expected := range []string{
		"appscript_list_projects",
		"appscript_get_project",
		"appscript_get_content",
		"appscript_list_deployments",
		"appscript_list_processes",
		"appscript_list_versions",
		"appscript_get_version",
		"appscript_get_metrics",
		"appscript_create_project",
		"appscript_update_content",
		"appscript_run_function",
		"appscript_create_deployment",
		"appscript_update_deployment",
		"appscript_delete_deployment",
		"appscript_delete_project",
		"appscript_create_version",
		"appscript_generate_trigger_code",
	} {
		if !names[expected] {
			t.Errorf("expected tool %q to be present", expected)
		}
	}

	// --tools appscript --tool-tier core: 7 core AppScript tools.
	s2 := newTestServer(t, server.Config{Tools: []string{"appscript"}, ToolTier: "core"})
	names2 := registeredToolNames(t, s2)
	if len(names2) != 7 {
		t.Errorf("expected 7 tools with --tools appscript --tool-tier core, got %d: %v", len(names2), names2)
	}

	// --tools appscript --read-only: 8 AppScript read-only tools (write tools removed).
	s3 := newTestServer(t, server.Config{Tools: []string{"appscript"}, ReadOnly: true})
	names3 := registeredToolNames(t, s3)
	if len(names3) != 8 {
		t.Errorf("expected 8 tools with --tools appscript --read-only, got %d: %v", len(names3), names3)
	}
}

func TestStartGoogleAuthOAuth21Disabled(t *testing.T) {
	// When MCP_ENABLE_OAUTH21 is not set, auth_start should be registered.
	t.Setenv("MCP_ENABLE_OAUTH21", "")
	s := newTestServer(t, server.Config{})
	names := registeredToolNames(t, s)
	if !names["auth_start"] {
		t.Error("expected auth_start to be registered when MCP_ENABLE_OAUTH21 is not set")
	}
}

func TestStartGoogleAuthOAuth21Enabled(t *testing.T) {
	// When MCP_ENABLE_OAUTH21=true, auth_start should NOT be registered.
	t.Setenv("MCP_ENABLE_OAUTH21", "true")
	s := server.New(server.Config{})
	RegisterAllTools(s, server.Config{})
	FilterTools(s, server.Config{})
	names := registeredToolNames(t, s)
	if names["auth_start"] {
		t.Error("expected auth_start to NOT be registered when MCP_ENABLE_OAUTH21=true")
	}
	// Should have 137 tools (138 - 1 auth_start).
	if len(names) != 137 {
		t.Errorf("expected 137 tools with MCP_ENABLE_OAUTH21=true, got %d", len(names))
	}
}

func TestAllowedToolsForTier(t *testing.T) {
	tests := []struct {
		tier     string
		wantNil  bool
		wantTool string
		wantIn   bool
	}{
		{"", true, "", false},
		{"complete", true, "", false},
		{"core", false, "gmail_search_messages", true},
		{"core", false, "gmail_get_attachment", false},
		{"extended", false, "gmail_search_messages", true},
		{"extended", false, "gmail_get_attachment", true},
		{"extended", false, "gmail_get_threads_batch", false},
	}

	for _, tt := range tests {
		allowed := allowedToolsForTier(tt.tier)
		if tt.wantNil {
			if allowed != nil {
				t.Errorf("tier %q: expected nil, got %v", tt.tier, allowed)
			}
			continue
		}
		if allowed == nil {
			t.Errorf("tier %q: expected non-nil", tt.tier)
			continue
		}
		if got := allowed[tt.wantTool]; got != tt.wantIn {
			t.Errorf("tier %q, tool %q: want %v, got %v", tt.tier, tt.wantTool, tt.wantIn, got)
		}
	}
}
