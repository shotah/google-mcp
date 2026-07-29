package tools

import (
	"strings"
	"testing"
)

// --- drive_search_files ---

func TestDriveHandlerSearchMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_search_files", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error mentioning 'query', got %q", text)
	}
}

func TestDriveHandlerSearchAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_search_files", map[string]any{
		"query": "test document",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- drive_get_file_content ---

func TestDriveHandlerGetFileContentMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_get_file_content", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerGetFileContentAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_get_file_content", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- drive_get_file_download_url ---

func TestDriveHandlerGetFileDownloadURLMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_get_file_download_url", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

// --- drive_list_items ---
// drive_list_items has no strictly required params (folder_id defaults to "root"),
// so the first error path is auth failure.

func TestDriveHandlerListDriveItemsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_list_items", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") && !strings.Contains(lower, "token") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- drive_get_file_permissions ---

func TestDriveHandlerGetFilePermissionsMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_get_file_permissions", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

// --- drive_check_file_public_access ---

func TestDriveHandlerCheckPublicAccessMissingFileName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_check_file_public_access", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_name") {
		t.Errorf("expected error mentioning 'file_name', got %q", text)
	}
}

// --- drive_get_shareable_link ---

func TestDriveHandlerGetShareableLinkMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_get_shareable_link", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

// --- drive_create_file ---

func TestDriveHandlerCreateFileMissingFileName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_create_file", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_name") {
		t.Errorf("expected error mentioning 'file_name', got %q", text)
	}
}

func TestDriveHandlerCreateFileMissingContentAndURL(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_create_file", map[string]any{
		"file_name": "test.txt",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "content") && !strings.Contains(lower, "fileurl") {
		t.Errorf("expected error mentioning 'content' or 'fileUrl', got %q", text)
	}
}

// --- drive_import_to_doc ---

func TestDriveHandlerImportToGoogleDocMissingFileName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_import_to_doc", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_name") {
		t.Errorf("expected error mentioning 'file_name', got %q", text)
	}
}

func TestDriveHandlerImportToGoogleDocMissingSource(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_import_to_doc", map[string]any{
		"file_name": "test.md",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "content") && !strings.Contains(lower, "file_path") && !strings.Contains(lower, "file_url") {
		t.Errorf("expected error mentioning source params, got %q", text)
	}
}

// --- drive_update_file ---

func TestDriveHandlerUpdateFileMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_update_file", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

// --- drive_copy_file ---

func TestDriveHandlerCopyFileMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_copy_file", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

// --- drive_share_file ---

func TestDriveHandlerShareFileMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_share_file", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerShareFileMissingShareWith(t *testing.T) {
	s := newToolTestServer(t)
	// share_type defaults to "user", which requires share_with.
	// This param check happens before auth.
	text, isError := callTool(t, s, "drive_share_file", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "share_with") {
		t.Errorf("expected error mentioning 'share_with', got %q", text)
	}
}

// --- drive_batch_share_file ---

func TestDriveHandlerBatchShareFileMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_batch_share_file", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerBatchShareFileMissingRecipients(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_batch_share_file", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "recipients") {
		t.Errorf("expected error mentioning 'recipients', got %q", text)
	}
}

func TestDriveHandlerBatchShareFileEmptyRecipients(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_batch_share_file", map[string]any{
		"file_id":    "abc123",
		"recipients": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "recipients") && !strings.Contains(lower, "empty") {
		t.Errorf("expected error mentioning empty recipients, got %q", text)
	}
}

// --- drive_update_permission ---

func TestDriveHandlerUpdatePermissionMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_update_permission", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerUpdatePermissionMissingPermissionID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_update_permission", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "permission_id") {
		t.Errorf("expected error mentioning 'permission_id', got %q", text)
	}
}

func TestDriveHandlerUpdatePermissionMissingRoleAndExpiration(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_update_permission", map[string]any{
		"file_id":       "abc123",
		"permission_id": "perm456",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "role") && !strings.Contains(lower, "expiration") {
		t.Errorf("expected error mentioning 'role' or 'expiration_time', got %q", text)
	}
}

func TestDriveHandlerUpdatePermissionInvalidRole(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_update_permission", map[string]any{
		"file_id":       "abc123",
		"permission_id": "perm456",
		"role":          "superadmin",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "invalid role") {
		t.Errorf("expected error mentioning 'invalid role', got %q", text)
	}
}

// --- drive_remove_permission ---

func TestDriveHandlerRemovePermissionMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_remove_permission", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerRemovePermissionMissingPermissionID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_remove_permission", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "permission_id") {
		t.Errorf("expected error mentioning 'permission_id', got %q", text)
	}
}

// --- drive_transfer_ownership ---

func TestDriveHandlerTransferOwnershipMissingFileID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_transfer_ownership", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_id") {
		t.Errorf("expected error mentioning 'file_id', got %q", text)
	}
}

func TestDriveHandlerTransferOwnershipMissingNewOwnerEmail(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_transfer_ownership", map[string]any{
		"file_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "new_owner_email") {
		t.Errorf("expected error mentioning 'new_owner_email', got %q", text)
	}
}

// --- Additional validation tests ---

func TestDriveHandlerShareFileInvalidRole(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_share_file", map[string]any{
		"file_id":    "abc123",
		"share_with": "user@example.com",
		"role":       "superadmin",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "invalid role") {
		t.Errorf("expected error mentioning 'invalid role', got %q", text)
	}
}

func TestDriveHandlerShareFileInvalidShareType(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_share_file", map[string]any{
		"file_id":    "abc123",
		"share_with": "user@example.com",
		"share_type": "martian",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "invalid share_type") {
		t.Errorf("expected error mentioning 'invalid share_type', got %q", text)
	}
}

func TestDriveHandlerImportToGoogleDocMultipleSources(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_import_to_doc", map[string]any{
		"file_name": "test.md",
		"content":   "# Hello",
		"file_path": "/tmp/test.md",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "only one") && !strings.Contains(lower, "provide only") {
		t.Errorf("expected error about providing only one source, got %q", text)
	}
}

func TestDriveHandlerImportToGoogleDocUnsupportedFormat(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "drive_import_to_doc", map[string]any{
		"file_name":     "test.xyz",
		"content":       "some content",
		"source_format": "xyz",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "unsupported") {
		t.Errorf("expected error mentioning 'unsupported', got %q", text)
	}
}
