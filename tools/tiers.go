package tools

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/server"
)

// tierTools maps each service to its tier breakdown (core, extended, complete).
// Tool names are service-first ({service}_{verb}_{object}) for agent clarity.
var tierTools = map[string]map[string][]string{
	"gmail": {
		"core": {
			"gmail_search_messages",
			"gmail_get_message",
			"gmail_get_messages_batch",
			"gmail_send_message",
			"gmail_modify_message_labels", // trash/archive/spam — everyday cleanup
		},
		"extended": {
			"gmail_get_attachment",
			"gmail_get_thread",
			"gmail_list_labels",
			"gmail_manage_label",
			"gmail_draft_message",
			"gmail_list_filters",
			"gmail_create_filter",
			"gmail_delete_filter",
		},
		"complete": {
			"gmail_get_threads_batch",
			"gmail_batch_modify_message_labels",
			"auth_start",
		},
	},
	"drive": {
		"core": {
			"drive_search_files",
			"drive_get_file_content",
			"drive_get_file_download_url",
			"drive_create_file",
			"drive_import_to_doc",
			"drive_share_file",
			"drive_get_shareable_link",
		},
		"extended": {
			"drive_list_items",
			"drive_copy_file",
			"drive_update_file",
			"drive_update_permission",
			"drive_remove_permission",
			"drive_transfer_ownership",
			"drive_batch_share_file",
		},
		"complete": {
			"drive_get_file_permissions",
			"drive_check_file_public_access",
		},
	},
	"calendar": {
		"core": {
			"calendar_list_calendars",
			"calendar_list_events",
			"calendar_get_event",
			"calendar_create_event",
			"calendar_update_event",
			"calendar_delete_event", // everyday dedupe / cleanup — keep in core+edit
		},
		"extended": {
			"calendar_query_freebusy",
		},
		"complete": {},
	},
	"docs": {
		"core": {
			"docs_get_content",
			"docs_create",
			"docs_modify_text",
		},
		"extended": {
			"docs_export_to_pdf",
			"docs_search",
			"docs_find_and_replace",
			"docs_list_in_folder",
			"docs_insert_elements",
			"docs_update_paragraph_style",
		},
		"complete": {
			"docs_insert_image",
			"docs_update_headers_footers",
			"docs_batch_update",
			"docs_inspect_structure",
			"docs_create_table_with_data",
			"docs_debug_table_structure",
			"docs_read_comments",
			"docs_create_comment",
			"docs_reply_to_comment",
			"docs_resolve_comment",
		},
	},
	"sheets": {
		"core": {
			"sheets_create_spreadsheet",
			"sheets_read_values",
			"sheets_modify_values",
		},
		"extended": {
			"sheets_list_spreadsheets",
			"sheets_get_spreadsheet_info",
		},
		"complete": {
			"sheets_create_sheet",
			"sheets_format_range",
			"sheets_add_conditional_formatting",
			"sheets_update_conditional_formatting",
			"sheets_delete_conditional_formatting",
			"sheets_read_comments",
			"sheets_create_comment",
			"sheets_reply_to_comment",
			"sheets_resolve_comment",
		},
	},
	"chat": {
		"core": {
			"chat_send_message",
			"chat_list_messages",
			"chat_search_messages",
		},
		"extended": {
			"chat_list_spaces",
		},
		"complete": {},
	},
	"forms": {
		"core": {
			"forms_create",
			"forms_get",
		},
		"extended": {
			"forms_list_responses",
		},
		"complete": {
			"forms_set_publish_settings",
			"forms_get_response",
			"forms_batch_update",
		},
	},
	"slides": {
		"core": {
			"slides_create_presentation",
			"slides_get_presentation",
		},
		"extended": {
			"slides_batch_update",
			"slides_get_page",
			"slides_get_page_thumbnail",
		},
		"complete": {
			"slides_read_comments",
			"slides_create_comment",
			"slides_reply_to_comment",
			"slides_resolve_comment",
		},
	},
	"tasks": {
		"core": {
			"tasks_get_task",
			"tasks_list_tasks",
			"tasks_create_task",
			"tasks_update_task",
			// "make me a grocery list" needs a new list — everyday assistant path.
			"tasks_list_tasklists",
			"tasks_create_tasklist",
		},
		"extended": {
			"tasks_delete_task",
		},
		"complete": {
			"tasks_get_tasklist",
			"tasks_update_tasklist",
			"tasks_delete_tasklist",
			"tasks_move_task",
			"tasks_clear_completed",
		},
	},
	"contacts": {
		"core": {
			"contacts_search",
			"contacts_get",
			"contacts_list",
			"contacts_create",
		},
		"extended": {
			"contacts_update",
			"contacts_delete",
			"contacts_list_groups",
			"contacts_get_group",
		},
		"complete": {
			"contacts_batch_create",
			"contacts_batch_update",
			"contacts_batch_delete",
			"contacts_create_group",
			"contacts_update_group",
			"contacts_delete_group",
			"contacts_modify_group_members",
		},
	},
	"search": {
		"core": {
			"search_query",
		},
		"extended": {
			"search_query_siterestrict",
		},
		"complete": {
			"search_get_engine_info",
		},
	},
	"appscript": {
		"core": {
			"appscript_list_projects",
			"appscript_get_project",
			"appscript_get_content",
			"appscript_create_project",
			"appscript_update_content",
			"appscript_run_function",
			"appscript_generate_trigger_code",
		},
		"extended": {
			"appscript_create_deployment",
			"appscript_list_deployments",
			"appscript_update_deployment",
			"appscript_delete_deployment",
			"appscript_delete_project",
			"appscript_list_versions",
			"appscript_create_version",
			"appscript_get_version",
			"appscript_list_processes",
			"appscript_get_metrics",
		},
		"complete": {},
	},
}

// destructiveTools are withheld unless --capability complete (or unset).
// Everyday deletes (events, tasks, contacts, filters) stay available under edit.
var destructiveTools = map[string]bool{
	"drive_transfer_ownership": true,
	"contacts_batch_delete":    true,
	"tasks_delete_tasklist":    true,
	"contacts_delete_group":    true,
	"appscript_delete_project": true,
	"tasks_clear_completed":    true,
}

// readOnlyTools is the set of tools that are allowed in --read-only mode /
// --capability read. All other tools require write scopes.
var readOnlyTools = map[string]bool{
	// Gmail
	"gmail_search_messages":    true,
	"gmail_get_message":        true,
	"gmail_get_messages_batch": true,
	"gmail_get_attachment":     true,
	"gmail_get_thread":         true,
	"gmail_get_threads_batch":  true,
	"gmail_list_labels":        true,
	"gmail_list_filters":       true,
	// Drive
	"drive_search_files":             true,
	"drive_get_file_content":         true,
	"drive_get_file_download_url":    true,
	"drive_list_items":               true,
	"drive_get_file_permissions":     true,
	"drive_check_file_public_access": true,
	"drive_get_shareable_link":       true,
	// Calendar
	"calendar_list_calendars": true,
	"calendar_list_events":    true,
	"calendar_get_event":      true,
	"calendar_query_freebusy": true,
	// Docs
	"docs_search":                true,
	"docs_get_content":           true,
	"docs_list_in_folder":        true,
	"docs_inspect_structure":     true,
	"docs_debug_table_structure": true,
	"docs_export_to_pdf":         true,
	// Sheets
	"sheets_list_spreadsheets":    true,
	"sheets_get_spreadsheet_info": true,
	"sheets_read_values":          true,
	// Chat
	"chat_list_spaces":     true,
	"chat_list_messages":   true,
	"chat_search_messages": true,
	// Forms
	"forms_get":            true,
	"forms_get_response":   true,
	"forms_list_responses": true,
	// Slides
	"slides_get_presentation":   true,
	"slides_get_page":           true,
	"slides_get_page_thumbnail": true,
	// Tasks
	"tasks_get_task":       true,
	"tasks_list_tasks":     true,
	"tasks_list_tasklists": true,
	"tasks_get_tasklist":   true,
	// Contacts
	"contacts_search":      true,
	"contacts_get":         true,
	"contacts_list":        true,
	"contacts_list_groups": true,
	"contacts_get_group":   true,
	// Search (all read-only)
	"search_query":              true,
	"search_get_engine_info":    true,
	"search_query_siterestrict": true,
	// Apps Script
	"appscript_list_projects":    true,
	"appscript_get_project":      true,
	"appscript_get_content":      true,
	"appscript_list_deployments": true,
	"appscript_list_processes":   true,
	"appscript_list_versions":    true,
	"appscript_get_version":      true,
	"appscript_get_metrics":      true,
	// Comments (read only)
	"docs_read_comments":   true,
	"sheets_read_comments": true,
	"slides_read_comments": true,
}

// allowedToolsForTier returns the set of tool names allowed for the given tier.
// "core" = core only, "extended" = core + extended, "complete" (or empty) = all.
func allowedToolsForTier(tier string) map[string]bool {
	if tier == "" || tier == "complete" {
		return nil // nil means allow all
	}

	allowed := make(map[string]bool)
	for _, service := range tierTools {
		for _, name := range service["core"] {
			allowed[name] = true
		}
		if tier == "extended" {
			for _, name := range service["extended"] {
				allowed[name] = true
			}
		}
	}
	return allowed
}

// capabilityAllows reports whether tool name is permitted for the capability.
// Empty capability means complete (allow everything).
func capabilityAllows(capability, name string) bool {
	switch capability {
	case "", "complete":
		return true
	case "read":
		return readOnlyTools[name]
	case "edit":
		return !destructiveTools[name]
	default:
		return true
	}
}

// FilterTools removes tools from the server based on tier, capability, and
// read-only settings. It is called after RegisterAllTools.
func FilterTools(s *mcpserver.MCPServer, cfg server.Config) {
	tierAllowed := allowedToolsForTier(cfg.ToolTier)
	capability := cfg.Capability
	if cfg.ReadOnly {
		// --read-only is the stricter shorthand for --capability read.
		capability = "read"
	}

	var toRemove []string

	// Collect all known tool names from tier definitions.
	allKnown := make(map[string]bool)
	for _, service := range tierTools {
		for _, names := range service {
			for _, name := range names {
				allKnown[name] = true
			}
		}
	}

	for name := range allKnown {
		remove := false

		// Tier filtering: remove tools not in the allowed tier set.
		if tierAllowed != nil && !tierAllowed[name] {
			remove = true
		}

		// Capability filtering: read / edit / complete.
		if !capabilityAllows(capability, name) {
			remove = true
		}

		if remove {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) > 0 {
		s.DeleteTools(toRemove...)
	}
}
