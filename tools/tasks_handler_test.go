package tools

import (
	"strings"
	"testing"
)

// --- tasks_list_tasklists ---
// tasks_list_tasklists has no strictly required params (email resolved via env),
// so the first error path is auth failure.

func TestTasksHandlerListTaskListsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_list_tasklists", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- tasks_get_tasklist ---

func TestTasksHandlerGetTaskListMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_get_tasklist", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerGetTaskListAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_get_tasklist", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- tasks_create_tasklist ---

func TestTasksHandlerCreateTaskListMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_create_tasklist", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

// --- tasks_update_tasklist ---

func TestTasksHandlerUpdateTaskListMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_update_tasklist", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerUpdateTaskListMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_update_tasklist", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

// --- tasks_delete_tasklist ---

func TestTasksHandlerDeleteTaskListMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_delete_tasklist", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

// --- tasks_list_tasks ---

func TestTasksHandlerListTasksMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_list_tasks", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

// --- tasks_get_task ---

func TestTasksHandlerGetTaskMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_get_task", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerGetTaskMissingTaskID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_get_task", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_id") {
		t.Errorf("expected error mentioning 'task_id', got %q", text)
	}
}

// --- tasks_create_task ---

func TestTasksHandlerCreateTaskMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_create_task", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerCreateTaskMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_create_task", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

// --- tasks_update_task ---

func TestTasksHandlerUpdateTaskMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_update_task", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerUpdateTaskMissingTaskID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_update_task", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_id") {
		t.Errorf("expected error mentioning 'task_id', got %q", text)
	}
}

// --- tasks_delete_task ---

func TestTasksHandlerDeleteTaskMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_delete_task", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerDeleteTaskMissingTaskID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_delete_task", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_id") {
		t.Errorf("expected error mentioning 'task_id', got %q", text)
	}
}

// --- tasks_move_task ---

func TestTasksHandlerMoveTaskMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_move_task", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerMoveTaskMissingTaskID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_move_task", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_id") {
		t.Errorf("expected error mentioning 'task_id', got %q", text)
	}
}

// --- tasks_clear_completed ---

func TestTasksHandlerClearCompletedTasksMissingTaskListID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_clear_completed", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "task_list_id") {
		t.Errorf("expected error mentioning 'task_list_id', got %q", text)
	}
}

func TestTasksHandlerClearCompletedTasksAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "tasks_clear_completed", map[string]any{
		"task_list_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}
