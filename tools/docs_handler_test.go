package tools

import (
	"strings"
	"testing"
)

// --- docs_search ---

func TestDocsHandlerSearchDocsMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_search", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error mentioning 'query', got %q", text)
	}
}

func TestDocsHandlerSearchDocsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_search", map[string]any{
		"query": "test doc",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- docs_get_content ---

func TestDocsHandlerGetDocContentMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_get_content", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerGetDocContentAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_get_content", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- docs_list_in_folder ---
// docs_list_in_folder has no strictly required params (folder_id defaults to "root"),
// so the first error path is auth failure.

func TestDocsHandlerListDocsInFolderAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_list_in_folder", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- docs_create ---

func TestDocsHandlerCreateDocMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

func TestDocsHandlerCreateDocAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create", map[string]any{
		"title": "Test Document",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- docs_inspect_structure ---

func TestDocsHandlerInspectDocStructureMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_inspect_structure", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

// --- docs_debug_table_structure ---

func TestDocsHandlerDebugTableStructureMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_debug_table_structure", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

// --- docs_export_to_pdf ---

func TestDocsHandlerExportDocToPDFMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_export_to_pdf", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

// --- docs_modify_text ---

func TestDocsHandlerModifyDocTextMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_modify_text", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerModifyDocTextMissingTextAndFormatting(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_modify_text", map[string]any{
		"document_id": "doc123",
		"start_index": 1,
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "text") {
		t.Errorf("expected error mentioning 'text', got %q", text)
	}
	if !strings.Contains(lower, "next:") {
		t.Errorf("expected teach-in next call, got %q", text)
	}
}

// --- docs_find_and_replace ---

func TestDocsHandlerFindAndReplaceDocMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_find_and_replace", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerFindAndReplaceDocMissingFindText(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_find_and_replace", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "find_text") {
		t.Errorf("expected error mentioning 'find_text', got %q", text)
	}
}

func TestDocsHandlerFindAndReplaceDocMissingReplaceText(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_find_and_replace", map[string]any{
		"document_id": "doc123",
		"find_text":   "hello",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "replace_text") {
		t.Errorf("expected error mentioning 'replace_text', got %q", text)
	}
}

// --- docs_insert_elements ---

func TestDocsHandlerInsertDocElementsMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_insert_elements", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerInsertDocElementsMissingElementType(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_insert_elements", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "element_type") {
		t.Errorf("expected error mentioning 'element_type', got %q", text)
	}
}

// --- docs_insert_image ---

func TestDocsHandlerInsertDocImageMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_insert_image", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerInsertDocImageMissingImageSource(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_insert_image", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "image_source") {
		t.Errorf("expected error mentioning 'image_source', got %q", text)
	}
}

// --- docs_update_headers_footers ---

func TestDocsHandlerUpdateDocHeadersFootersMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_update_headers_footers", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerUpdateDocHeadersFootersMissingSectionType(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_update_headers_footers", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "section_type") {
		t.Errorf("expected error mentioning 'section_type', got %q", text)
	}
}

func TestDocsHandlerUpdateDocHeadersFootersMissingContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_update_headers_footers", map[string]any{
		"document_id":  "doc123",
		"section_type": "header",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "content") {
		t.Errorf("expected error mentioning 'content', got %q", text)
	}
}

// --- docs_batch_update ---

func TestDocsHandlerBatchUpdateDocMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_batch_update", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerBatchUpdateDocMissingOperations(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_batch_update", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "operations") {
		t.Errorf("expected error mentioning 'operations', got %q", text)
	}
}

// --- docs_create_table_with_data ---

func TestDocsHandlerCreateTableWithDataMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create_table_with_data", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerCreateTableWithDataMissingTableData(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create_table_with_data", map[string]any{
		"document_id": "doc123",
		"index":       1,
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "table_data") {
		t.Errorf("expected error mentioning 'table_data', got %q", text)
	}
}

// --- docs_update_paragraph_style ---

func TestDocsHandlerUpdateParagraphStyleMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_update_paragraph_style", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerUpdateParagraphStyleInvalidStartIndex(t *testing.T) {
	s := newToolTestServer(t)
	// start_index defaults to 0 via GetInt, which is < 1 → validation error
	text, isError := callTool(t, s, "docs_update_paragraph_style", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "start_index") {
		t.Errorf("expected error mentioning 'start_index', got %q", text)
	}
}

func TestDocsHandlerUpdateParagraphStyleInvalidEndIndex(t *testing.T) {
	s := newToolTestServer(t)
	// end_index defaults to 0 via GetInt, which is <= start_index (1)
	text, isError := callTool(t, s, "docs_update_paragraph_style", map[string]any{
		"document_id": "doc123",
		"start_index": 1,
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "end_index") {
		t.Errorf("expected error mentioning 'end_index', got %q", text)
	}
}

// --- Document Comment Tools (via RegisterCommentTools) ---

func TestDocsHandlerReadDocumentCommentsMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_read_comments", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerCreateDocumentCommentMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerCreateDocumentCommentMissingContent(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_create_comment", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_content") {
		t.Errorf("expected error mentioning 'comment_content', got %q", text)
	}
}

func TestDocsHandlerReplyToDocumentCommentMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_reply_to_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}

func TestDocsHandlerReplyToDocumentCommentMissingCommentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_reply_to_comment", map[string]any{
		"document_id": "doc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "comment_id") {
		t.Errorf("expected error mentioning 'comment_id', got %q", text)
	}
}

func TestDocsHandlerResolveDocumentCommentMissingDocumentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "docs_resolve_comment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "document_id") {
		t.Errorf("expected error mentioning 'document_id', got %q", text)
	}
}
