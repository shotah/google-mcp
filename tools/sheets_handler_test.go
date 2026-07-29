package tools

import (
	"strings"
	"testing"
)

// --- sheets_list_spreadsheets ---
// sheets_list_spreadsheets has no strictly required params (email resolved via env),
// so the first error path is auth failure.

func TestSheetsHandlerListSpreadsheetsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_list_spreadsheets", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- sheets_get_spreadsheet_info ---

func TestSheetsHandlerGetSpreadsheetInfoMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_get_spreadsheet_info", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerGetSpreadsheetInfoAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_get_spreadsheet_info", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- sheets_read_values ---

func TestSheetsHandlerReadSheetValuesMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_read_values", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

// --- sheets_modify_values ---

func TestSheetsHandlerModifySheetValuesMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_modify_values", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerModifySheetValuesMissingRangeName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_modify_values", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "range_name") {
		t.Errorf("expected error mentioning 'range_name', got %q", text)
	}
}

// --- sheets_format_range ---

func TestSheetsHandlerFormatSheetRangeMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_format_range", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerFormatSheetRangeMissingRangeName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_format_range", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "range_name") {
		t.Errorf("expected error mentioning 'range_name', got %q", text)
	}
}

func TestSheetsHandlerFormatSheetRangeMissingFormattingOptions(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_format_range", map[string]any{
		"spreadsheet_id": "sheet123",
		"range_name":     "A1:B2",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "background_color") && !strings.Contains(lower, "text_color") && !strings.Contains(lower, "number_format") {
		t.Errorf("expected error mentioning formatting options, got %q", text)
	}
}

// --- sheets_add_conditional_formatting ---

func TestSheetsHandlerAddConditionalFormattingMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_add_conditional_formatting", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerAddConditionalFormattingMissingRangeName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_add_conditional_formatting", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "range_name") {
		t.Errorf("expected error mentioning 'range_name', got %q", text)
	}
}

func TestSheetsHandlerAddConditionalFormattingMissingConditionType(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_add_conditional_formatting", map[string]any{
		"spreadsheet_id": "sheet123",
		"range_name":     "A1:B2",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "condition_type") {
		t.Errorf("expected error mentioning 'condition_type', got %q", text)
	}
}

// --- sheets_update_conditional_formatting ---

func TestSheetsHandlerUpdateConditionalFormattingMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_update_conditional_formatting", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerUpdateConditionalFormattingNegativeRuleIndex(t *testing.T) {
	s := newToolTestServer(t)
	// rule_index defaults to -1 via GetInt, which fails the non-negative check
	text, isError := callTool(t, s, "sheets_update_conditional_formatting", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "rule_index") {
		t.Errorf("expected error mentioning 'rule_index', got %q", text)
	}
}

// --- sheets_delete_conditional_formatting ---

func TestSheetsHandlerDeleteConditionalFormattingMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_delete_conditional_formatting", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerDeleteConditionalFormattingNegativeRuleIndex(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_delete_conditional_formatting", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "rule_index") {
		t.Errorf("expected error mentioning 'rule_index', got %q", text)
	}
}

// --- sheets_create_spreadsheet ---

func TestSheetsHandlerCreateSpreadsheetMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_spreadsheet", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

func TestSheetsHandlerCreateSpreadsheetAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_spreadsheet", map[string]any{
		"title": "Test Spreadsheet",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "authentication") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- sheets_create_sheet ---

func TestSheetsHandlerCreateSheetMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_sheet", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerCreateSheetMissingSheetName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_sheet", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "sheet_name") {
		t.Errorf("expected error mentioning 'sheet_name', got %q", text)
	}
}

// --- Spreadsheet Comment Tools (via RegisterCommentTools) ---

func TestSheetsHandlerReadSpreadsheetCommentsMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_read_comments", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerCreateSpreadsheetCommentMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerCreateSpreadsheetCommentMissingContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_create_comment", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_content") {
		t.Errorf("expected error mentioning 'comment_content', got %q", text)
	}
}

func TestSheetsHandlerReplyToSpreadsheetCommentMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_reply_to_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerReplyToSpreadsheetCommentMissingCommentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_reply_to_comment", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_id") {
		t.Errorf("expected error mentioning 'comment_id', got %q", text)
	}
}

func TestSheetsHandlerReplyToSpreadsheetCommentMissingReplyContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_reply_to_comment", map[string]any{
		"spreadsheet_id": "sheet123",
		"comment_id":     "comment123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "reply_content") {
		t.Errorf("expected error mentioning 'reply_content', got %q", text)
	}
}

func TestSheetsHandlerResolveSpreadsheetCommentMissingSpreadsheetID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_resolve_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "spreadsheet_id") {
		t.Errorf("expected error mentioning 'spreadsheet_id', got %q", text)
	}
}

func TestSheetsHandlerResolveSpreadsheetCommentMissingCommentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "sheets_resolve_comment", map[string]any{
		"spreadsheet_id": "sheet123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_id") {
		t.Errorf("expected error mentioning 'comment_id', got %q", text)
	}
}
