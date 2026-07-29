package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// --- tasks_list_tasklists ---

func TestTasksMockListTaskLists(t *testing.T) {
	t.Run("success_with_lists", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/tasks/v1/users/@me/lists": map[string]any{
				"items": []map[string]any{
					{"id": "tl001", "title": "Personal", "updated": "2026-02-18T10:00:00Z", "selfLink": "https://tasks.googleapis.com/tasks/v1/users/@me/lists/tl001"},
					{"id": "tl002", "title": "Work", "updated": "2026-02-17T08:00:00Z", "selfLink": "https://tasks.googleapis.com/tasks/v1/users/@me/lists/tl002"},
				},
			},
		})
		handler := handleListTaskLists(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Task Lists") {
			t.Errorf("expected 'Task Lists' in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Personal") {
			t.Errorf("expected 'Personal' in output")
		}
		if !strings.Contains(text, "Work") {
			t.Errorf("expected 'Work' in output")
		}
		if !strings.Contains(text, "tl001") {
			t.Errorf("expected task list ID in output")
		}
	})

	t.Run("success_no_lists", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/tasks/v1/users/@me/lists": map[string]any{
				"items": []map[string]any{},
			},
		})
		handler := handleListTaskLists(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No task lists found") {
			t.Errorf("expected 'No task lists found', got:\n%s", text)
		}
	})
}

// --- tasks_create_tasklist ---

func TestTasksMockCreateTaskList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/tasks/v1/users/@me/lists": map[string]any{
				"id":       "tl_new",
				"title":    "Shopping",
				"updated":  "2026-02-18T12:00:00Z",
				"selfLink": "https://tasks.googleapis.com/tasks/v1/users/@me/lists/tl_new",
			},
		})
		handler := handleCreateTaskList(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"title":             "Shopping",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Task List Created") {
			t.Errorf("expected 'Task List Created', got:\n%s", text)
		}
		if !strings.Contains(text, "Shopping") {
			t.Errorf("expected title in output")
		}
		if !strings.Contains(text, "tl_new") {
			t.Errorf("expected task list ID in output")
		}
	})
}

// --- tasks_list_tasks ---

func TestTasksMockListTasks(t *testing.T) {
	t.Run("success_with_tasks", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/tasks/v1/lists/tl001/tasks": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"items": [
						{"id":"task001","title":"Buy groceries","status":"needsAction","updated":"2026-02-18T10:00:00Z"},
						{"id":"task002","title":"Clean house","status":"completed","updated":"2026-02-17T15:00:00Z","completed":"2026-02-17T15:00:00.000Z"}
					]
				}`)
			},
		})
		handler := handleListTasks(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"task_list_id":      "tl001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Buy groceries") {
			t.Errorf("expected task title in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Clean house") {
			t.Errorf("expected second task in output")
		}
	})
}

// --- tasks_create_task ---

func TestTasksMockCreateTask(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/tasks/v1/lists/tl001/tasks": map[string]any{
				"id":      "newtask001",
				"title":   "Write tests",
				"status":  "needsAction",
				"updated": "2026-02-18T14:00:00Z",
			},
		})
		handler := handleCreateTask(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"task_list_id":      "tl001",
			"title":             "Write tests",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Task Created") {
			t.Errorf("expected 'Task Created', got:\n%s", text)
		}
		if !strings.Contains(text, "Write tests") {
			t.Errorf("expected task title in output")
		}
		if !strings.Contains(text, "newtask001") {
			t.Errorf("expected task ID in output")
		}
	})
}

// --- tasks_get_task ---

func TestTasksMockGetTask(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/tasks/v1/lists/tl001/tasks/task001": map[string]any{
			"id":       "task001",
			"title":    "Prepare release",
			"status":   "needsAction",
			"updated":  "2026-02-18T14:00:00Z",
			"notes":    "Verify changelog",
			"due":      "2026-02-20T17:00:00.000Z",
			"parent":   "parent001",
			"position": "00000000000000000001",
		},
	})
	handler := handleGetTask(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"task_list_id":      "tl001",
		"task_id":           "task001",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Task Details") || !strings.Contains(text, "Prepare release") {
		t.Errorf("expected task details, got:\n%s", text)
	}
	if !strings.Contains(text, "Verify changelog") {
		t.Errorf("expected task notes, got:\n%s", text)
	}
}

// --- tasks_update_task ---

func TestTasksMockUpdateTask(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/tasks/v1/lists/tl001/tasks/task001": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				fmt.Fprint(w, `{"id":"task001","title":"Prepare release","status":"needsAction","notes":"Verify changelog","updated":"2026-02-18T14:00:00Z"}`)
			case http.MethodPut:
				fmt.Fprint(w, `{"id":"task001","title":"Publish release","status":"completed","notes":"Approved","updated":"2026-02-19T09:00:00Z"}`)
			default:
				t.Errorf("unexpected method %s", r.Method)
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		},
	})
	handler := handleUpdateTask(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"task_list_id":      "tl001",
		"task_id":           "task001",
		"title":             "Publish release",
		"status":            "completed",
		"notes":             "Approved",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Task Updated") || !strings.Contains(text, "Publish release") {
		t.Errorf("expected updated task details, got:\n%s", text)
	}
	if !strings.Contains(text, "completed") {
		t.Errorf("expected updated status, got:\n%s", text)
	}
}

// --- tasks_move_task ---

func TestTasksMockMoveTask(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/tasks/v1/lists/tl001/tasks/task001/move": map[string]any{
			"id":       "task001",
			"title":    "Prepare release",
			"status":   "needsAction",
			"updated":  "2026-02-19T09:00:00Z",
			"parent":   "parent001",
			"position": "00000000000000000002",
		},
	})
	handler := handleMoveTask(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"task_list_id":          "tl001",
		"task_id":               "task001",
		"parent":                "parent001",
		"previous":              "task000",
		"destination_task_list": "tl002",
		"user_google_email":     "test@example.com",
	})
	if !strings.Contains(text, "Task Moved") || !strings.Contains(text, "Prepare release") {
		t.Errorf("expected moved task details, got:\n%s", text)
	}
	if !strings.Contains(text, "moved to task list tl002") {
		t.Errorf("expected destination details, got:\n%s", text)
	}
}

// --- API error responses ---

func TestTasksMockAPIError(t *testing.T) {
	t.Run("list_task_lists_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/tasks/v1/users/@me/lists": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := handleListTaskLists(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "listing task lists") {
			t.Errorf("expected listing error, got:\n%s", text)
		}
	})

	t.Run("create_task_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/tasks/v1/lists/tl001/tasks": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleCreateTask(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"task_list_id":      "tl001",
			"title":             "Bad Task",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "creating task") {
			t.Errorf("expected creating task error, got:\n%s", text)
		}
	})
}
