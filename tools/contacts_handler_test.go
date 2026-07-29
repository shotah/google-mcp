package tools

import (
	"strings"
	"testing"
)

// --- contacts_list ---
// contacts_list has no strictly required params, so first error is auth failure.

func TestContactsHandlerListContactsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_list", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- contacts_get ---

func TestContactsHandlerGetContactMissingContactID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_get", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "contact_id") {
		t.Errorf("expected error mentioning 'contact_id', got %q", text)
	}
	if !strings.Contains(text, "Next:") || !strings.Contains(text, "contacts_search") {
		t.Errorf("expected Next: teach-in via contacts_search, got %q", text)
	}
}

func TestContactsHandlerGetContactAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_get", map[string]any{
		"contact_id": "c1234567890",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- contacts_search ---

func TestContactsHandlerSearchContactsMissingQuery(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_search", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error mentioning 'query', got %q", text)
	}
	if !strings.Contains(text, "Next:") {
		t.Errorf("expected Next: teach-in, got %q", text)
	}
}

// --- contacts_list_groups ---
// contacts_list_groups has no strictly required params, so first error is auth failure.

func TestContactsHandlerListContactGroupsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_list_groups", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- contacts_get_group ---

func TestContactsHandlerGetContactGroupMissingGroupID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_get_group", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "group_id") {
		t.Errorf("expected error mentioning 'group_id', got %q", text)
	}
}

// --- contacts_create ---

func TestContactsHandlerCreateContactAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_create", map[string]any{
		"given_name": "Test",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

func TestContactsHandlerCreateContactNoFields(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_create", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "field") && !strings.Contains(lower, "given_name") {
		t.Errorf("expected error about required fields, got %q", text)
	}
	if !strings.Contains(text, "Next:") {
		t.Errorf("expected Next: teach-in, got %q", text)
	}
}

// --- contacts_update ---

func TestContactsHandlerUpdateContactMissingContactID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_update", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "contact_id") {
		t.Errorf("expected error mentioning 'contact_id', got %q", text)
	}
}

// --- contacts_delete ---

func TestContactsHandlerDeleteContactMissingContactID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_delete", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "contact_id") {
		t.Errorf("expected error mentioning 'contact_id', got %q", text)
	}
}

// --- contacts_batch_create ---
// contacts is checked via args type assertion, not RequireString.

func TestContactsHandlerBatchCreateContactsNoContacts(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_batch_create", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "contact") {
		t.Errorf("expected error mentioning 'contact', got %q", text)
	}
}

func TestContactsHandlerBatchCreateContactsEmptyArray(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_batch_create", map[string]any{
		"contacts": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "contact") {
		t.Errorf("expected error mentioning 'contact', got %q", text)
	}
}

// --- contacts_batch_update ---

func TestContactsHandlerBatchUpdateContactsNoUpdates(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_batch_update", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "update") {
		t.Errorf("expected error mentioning 'update', got %q", text)
	}
}

// --- contacts_batch_delete ---

func TestContactsHandlerBatchDeleteContactsMissingContactIDs(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_batch_delete", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "contact_id") {
		t.Errorf("expected error mentioning 'contact_id', got %q", text)
	}
}

func TestContactsHandlerBatchDeleteContactsEmptyArray(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_batch_delete", map[string]any{
		"contact_ids": []any{},
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "contact") {
		t.Errorf("expected error mentioning 'contact', got %q", text)
	}
}

// --- contacts_create_group ---

func TestContactsHandlerCreateContactGroupMissingName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_create_group", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "name") {
		t.Errorf("expected error mentioning 'name', got %q", text)
	}
}

// --- contacts_update_group ---

func TestContactsHandlerUpdateContactGroupMissingGroupID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_update_group", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "group_id") {
		t.Errorf("expected error mentioning 'group_id', got %q", text)
	}
}

func TestContactsHandlerUpdateContactGroupMissingName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_update_group", map[string]any{
		"group_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "name") {
		t.Errorf("expected error mentioning 'name', got %q", text)
	}
}

// --- contacts_delete_group ---

func TestContactsHandlerDeleteContactGroupMissingGroupID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_delete_group", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "group_id") {
		t.Errorf("expected error mentioning 'group_id', got %q", text)
	}
}

// --- contacts_modify_group_members ---

func TestContactsHandlerModifyContactGroupMembersMissingGroupID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_modify_group_members", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "group_id") {
		t.Errorf("expected error mentioning 'group_id', got %q", text)
	}
}

func TestContactsHandlerModifyContactGroupMembersNoAddOrRemove(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "contacts_modify_group_members", map[string]any{
		"group_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "add_contact_ids") && !strings.Contains(lower, "remove_contact_ids") {
		t.Errorf("expected error mentioning add/remove contact IDs, got %q", text)
	}
}
