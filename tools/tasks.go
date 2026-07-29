package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

const (
	listTasksMaxResultsDefault = 20
	listTasksMaxResultsMax     = 10000
)

// RegisterTasksTools registers all Tasks tools with the MCP server.
func RegisterTasksTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools
	registerListTaskLists(s, getClient)
	registerGetTaskList(s, getClient)
	registerListTasks(s, getClient)
	registerGetTask(s, getClient)

	// Write tools
	registerCreateTaskList(s, getClient)
	registerUpdateTaskList(s, getClient)
	registerDeleteTaskList(s, getClient)
	registerCreateTask(s, getClient)
	registerUpdateTask(s, getClient)
	registerDeleteTask(s, getClient)
	registerMoveTask(s, getClient)
	registerClearCompletedTasks(s, getClient)
}

// newTasksService creates a tasks.Service for the given user email.
func newTasksService(ctx context.Context, getClient httpClientFunc, email string) (*tasks.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := tasks.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Tasks service: %w", err)
	}
	return svc, nil
}

// --- structuredTask for hierarchical display ---

type structuredTask struct {
	id                  string
	title               string
	status              string
	due                 string
	notes               string
	updated             string
	completed           string
	isPlaceholderParent bool
	subtasks            []*structuredTask
}

// getStructuredTasks converts a flat list of tasks into hierarchical structuredTask objects.
func getStructuredTasks(items []*tasks.Task) []*structuredTask {
	tasksById := make(map[string]*structuredTask, len(items))
	positionsById := make(map[string]int64, len(items))

	for _, t := range items {
		completed := ""
		if t.Completed != nil {
			completed = *t.Completed
		}
		tasksById[t.Id] = &structuredTask{
			id:        t.Id,
			title:     t.Title,
			status:    t.Status,
			due:       t.Due,
			notes:     t.Notes,
			updated:   t.Updated,
			completed: completed,
		}
		if t.Position != "" {
			// Position is a string of digits; parse as int for sorting.
			var pos int64
			if _, err := fmt.Sscanf(t.Position, "%d", &pos); err == nil {
				positionsById[t.Id] = pos
			}
		}
	}

	root := &structuredTask{id: "root", title: "Root"}

	for _, t := range items {
		st := tasksById[t.Id]
		parentID := t.Parent

		var parent *structuredTask
		if parentID == "" {
			parent = root
		} else if p, ok := tasksById[parentID]; ok {
			parent = p
		} else {
			// Orphaned subtask: create placeholder parent.
			parent = &structuredTask{id: parentID, isPlaceholderParent: true}
			tasksById[parentID] = parent
			root.subtasks = append(root.subtasks, parent)
		}
		parent.subtasks = append(parent.subtasks, st)
	}

	sortStructuredTasks(root, positionsById)
	return root.subtasks
}

func sortStructuredTasks(root *structuredTask, positionsById map[string]int64) {
	sort.SliceStable(root.subtasks, func(i, j int) bool {
		pi, oki := positionsById[root.subtasks[i].id]
		pj, okj := positionsById[root.subtasks[j].id]
		if !oki {
			pi = math.MaxInt64
		}
		if !okj {
			pj = math.MaxInt64
		}
		return pi < pj
	})
	for _, st := range root.subtasks {
		sortStructuredTasks(st, positionsById)
	}
}

func serializeTasks(structured []*structuredTask, level int) string {
	var sb strings.Builder
	placeholderCount := 0
	for _, t := range structured {
		indent := strings.Repeat("  ", level)
		bullet := "-"
		if level > 0 {
			bullet = "*"
		}
		title := t.title
		if title == "" {
			if t.isPlaceholderParent {
				title = "Unknown parent"
				placeholderCount++
			} else {
				title = "Untitled"
			}
		}
		fmt.Fprintf(&sb, "%s%s %s (ID: %s)\n", indent, bullet, title, t.id)
		fmt.Fprintf(&sb, "%s  Status: %s\n", indent, orNA(t.status))
		if t.due != "" {
			fmt.Fprintf(&sb, "%s  Due: %s\n", indent, t.due)
		}
		if t.notes != "" {
			n := t.notes
			if len(n) > 100 {
				n = n[:100] + "..."
			}
			fmt.Fprintf(&sb, "%s  Notes: %s\n", indent, n)
		}
		if t.completed != "" {
			fmt.Fprintf(&sb, "%s  Completed: %s\n", indent, t.completed)
		}
		fmt.Fprintf(&sb, "%s  Updated: %s\n", indent, orNA(t.updated))
		sb.WriteString("\n")

		sb.WriteString(serializeTasks(t.subtasks, level+1))
	}

	if placeholderCount > 0 && level == 0 {
		fmt.Fprintf(&sb, "\n%d tasks with title Unknown parent are included as placeholders.\n", placeholderCount)
		sb.WriteString("These placeholders contain subtasks whose parents were not present in the task list.\n")
		sb.WriteString("This can occur due to pagination. Callers can often avoid this problem if max_results is large enough to contain all tasks (subtasks and their parents) without paging.\n")
		sb.WriteString("This can also occur due to filtering that excludes parent tasks while including their subtasks or due to deleted or hidden parent tasks.\n")
	}

	return sb.String()
}

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// adjustDueMaxForTasksAPI compensates for the Google Tasks API treating dueMax
// as an exclusive bound by adding one day.
func adjustDueMaxForTasksAPI(dueMax string) string {
	// Try RFC3339
	t, err := time.Parse(time.RFC3339, dueMax)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", dueMax)
		if err != nil {
			return dueMax
		}
		t = t.UTC()
	}
	adjusted := t.Add(24 * time.Hour)
	return adjusted.Format(time.RFC3339)
}

// --- tasks_list_tasklists ---

func registerListTaskLists(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_list_tasklists",
		mcp.WithDescription("List Google Tasks lists (tasklist_id + title). Use for 'what lists do I have' before tasks_list_tasks. Not for tasks inside a list — use tasks_list_tasks."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithNumber("max_results", mcp.Description("Maximum number of task lists to return (default: 1000, max: 1000).")),
		mcp.WithString("page_token", mcp.Description("Token for pagination.")),
	)
	s.AddTool(tool, handleListTaskLists(getClient))
}

func handleListTaskLists(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResults := request.GetInt("max_results", 1000)
		pageToken := request.GetString("page_token", "")

		call := svc.Tasklists.List().MaxResults(int64(maxResults))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing task lists: %v", err)), nil
		}

		if len(result.Items) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No task lists found for %s.", email)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Task Lists for %s:\n", email)
		for _, tl := range result.Items {
			fmt.Fprintf(&sb, "- %s (ID: %s)\n", tl.Title, tl.Id)
			fmt.Fprintf(&sb, "  Updated: %s\n", orNA(tl.Updated))
		}

		if result.NextPageToken != "" {
			fmt.Fprintf(&sb, "\nNext page token: %s", result.NextPageToken)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_get_tasklist ---

func registerGetTaskList(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_get_tasklist",
		mcp.WithDescription("Get one task list by tasklist_id (title, etag). Use after tasks_list_tasklists. Not for individual tasks — use tasks_get_task / tasks_list_tasks."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("The ID of the task list to retrieve.")),
	)
	s.AddTool(tool, handleGetTaskList(getClient))
}

func handleGetTaskList(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		tl, err := svc.Tasklists.Get(taskListID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting task list: %v", err)), nil
		}

		response := fmt.Sprintf(`Task List Details for %s:
- Title: %s
- ID: %s
- Updated: %s
- Self Link: %s`, email, tl.Title, tl.Id, orNA(tl.Updated), orNA(tl.SelfLink))

		return mcp.NewToolResultText(response), nil
	}
}

// --- tasks_create_tasklist ---

func registerCreateTaskList(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_create_tasklist",
		mcp.WithDescription("Create a new Google Tasks list. Use for 'make a list called…'. Returns tasklist_id for tasks_create_task / tasks_list_tasks."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("The title of the new task list.")),
	)
	s.AddTool(tool, handleCreateTaskList(getClient))
}

func handleCreateTaskList(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := svc.Tasklists.Insert(&tasks.TaskList{Title: title}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating task list: %v", err)), nil
		}

		response := fmt.Sprintf(`Task List Created for %s:
- Title: %s
- ID: %s
- Created: %s
- Self Link: %s`, email, result.Title, result.Id, orNA(result.Updated), orNA(result.SelfLink))

		return mcp.NewToolResultText(response), nil
	}
}

// --- tasks_update_tasklist ---

func registerUpdateTaskList(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_update_tasklist",
		mcp.WithDescription("Rename or update a task list. Required tasklist_id from tasks_list_tasklists. Not for tasks — use tasks_update_task."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("The ID of the task list to update.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("The new title for the task list.")),
	)
	s.AddTool(tool, handleUpdateTaskList(getClient))
}

func handleUpdateTaskList(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := &tasks.TaskList{Id: taskListID, Title: title}
		result, err := svc.Tasklists.Update(taskListID, body).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating task list: %v", err)), nil
		}

		response := fmt.Sprintf(`Task List Updated for %s:
- Title: %s
- ID: %s
- Updated: %s`, email, result.Title, result.Id, orNA(result.Updated))

		return mcp.NewToolResultText(response), nil
	}
}

// --- tasks_delete_tasklist ---

func registerDeleteTaskList(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_delete_tasklist",
		mcp.WithDescription("Delete a task list and ALL its tasks. Required tasklist_id. Destructive — confirm when unclear. Not for single tasks — use tasks_delete_task."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("The ID of the task list to delete.")),
	)
	s.AddTool(tool, handleDeleteTaskList(getClient))
}

func handleDeleteTaskList(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = svc.Tasklists.Delete(taskListID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting task list: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Task list %s has been deleted for %s. All tasks in this list have also been deleted.",
			taskListID, email,
		)), nil
	}
}

// --- tasks_list_tasks ---

func registerListTasks(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_list_tasks",
		mcp.WithDescription("List tasks in a list (title, due, status, task_id). Use for 'what's on my to-do'. Prefer task_list_id=\"@default\". Not for list names — use tasks_list_tasklists (complete tier)."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("Task list id. Use \"@default\" for the account default list.")),
		mcp.WithNumber("max_results", mcp.Description("Max tasks. Default: 20.")),
	)
	s.AddTool(tool, handleListTasks(getClient))
}

func handleListTasks(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `tasks_list_tasks(task_list_id="@default")`), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return needArg("task_list_id", `tasks_list_tasks(task_list_id="@default")`), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResults := request.GetInt("max_results", listTasksMaxResultsDefault)
		pageToken := request.GetString("page_token", "")

		args := request.GetArguments()
		showCompleted := getBool(request, "show_completed", true)
		showDeleted := getBool(request, "show_deleted", false)
		showHidden := getBool(request, "show_hidden", false)
		showAssigned := getBool(request, "show_assigned", false)

		completedMax := request.GetString("completed_max", "")
		completedMin := request.GetString("completed_min", "")
		dueMax := request.GetString("due_max", "")
		dueMin := request.GetString("due_min", "")
		updatedMin := request.GetString("updated_min", "")

		// Suppress unused variable warning for args.
		_ = args

		call := svc.Tasks.List(taskListID).MaxResults(int64(maxResults))
		call = call.ShowCompleted(showCompleted)
		call = call.ShowDeleted(showDeleted)
		call = call.ShowHidden(showHidden)
		call = call.ShowAssigned(showAssigned)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		if completedMax != "" {
			call = call.CompletedMax(completedMax)
		}
		if completedMin != "" {
			call = call.CompletedMin(completedMin)
		}
		if dueMax != "" {
			call = call.DueMax(adjustDueMaxForTasksAPI(dueMax))
		}
		if dueMin != "" {
			call = call.DueMin(dueMin)
		}
		if updatedMin != "" {
			call = call.UpdatedMin(updatedMin)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing tasks: %v", err)), nil
		}

		allTasks := result.Items
		nextPageToken := result.NextPageToken

		// Multi-page retrieval to support sorting and structured display.
		remaining := min(maxResults, listTasksMaxResultsMax) - len(allTasks)
		for remaining > 0 && nextPageToken != "" {
			call2 := svc.Tasks.List(taskListID).MaxResults(int64(remaining)).PageToken(nextPageToken)
			call2 = call2.ShowCompleted(showCompleted).ShowDeleted(showDeleted).ShowHidden(showHidden).ShowAssigned(showAssigned)
			if completedMax != "" {
				call2 = call2.CompletedMax(completedMax)
			}
			if completedMin != "" {
				call2 = call2.CompletedMin(completedMin)
			}
			if dueMax != "" {
				call2 = call2.DueMax(adjustDueMaxForTasksAPI(dueMax))
			}
			if dueMin != "" {
				call2 = call2.DueMin(dueMin)
			}
			if updatedMin != "" {
				call2 = call2.UpdatedMin(updatedMin)
			}

			more, err := call2.Do()
			if err != nil {
				break
			}
			if len(more.Items) == 0 {
				break
			}
			allTasks = append(allTasks, more.Items...)
			nextPageToken = more.NextPageToken
			remaining -= len(more.Items)
		}

		if len(allTasks) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No tasks found in task list %s for %s.", taskListID, email)), nil
		}

		structured := getStructuredTasks(allTasks)

		var sb strings.Builder
		fmt.Fprintf(&sb, "Tasks in list %s for %s:\n", taskListID, email)
		sb.WriteString(serializeTasks(structured, 0))

		if nextPageToken != "" {
			fmt.Fprintf(&sb, "Next page token: %s\n", nextPageToken)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_get_task ---

func registerGetTask(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_get_task",
		mcp.WithDescription("Get one task (title, due, notes, status). Use before update/delete. Prefer tasks_list_tasks first for task_id. Required: task_list_id + task_id."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("Task list id. Use \"@default\" for the account default list.")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task id from tasks_list_tasks.")),
	)
	s.AddTool(tool, handleGetTask(getClient))
}

func handleGetTask(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `tasks_get_task(task_list_id="@default", task_id=…)`), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return needArg("task_list_id", `tasks_list_tasks(task_list_id="@default") then tasks_get_task(task_list_id, task_id)`), nil
		}
		taskID, err := request.RequireString("task_id")
		if err != nil {
			return needArg("task_id", `tasks_list_tasks(task_list_id="@default") then tasks_get_task(task_list_id, task_id)`), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		task, err := svc.Tasks.Get(taskListID, taskID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting task: %v", err)), nil
		}

		title := task.Title
		if title == "" {
			title = "Untitled"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Task Details for %s:\n", email)
		fmt.Fprintf(&sb, "- Title: %s\n", title)
		fmt.Fprintf(&sb, "- ID: %s\n", task.Id)
		fmt.Fprintf(&sb, "- Status: %s\n", orNA(task.Status))
		fmt.Fprintf(&sb, "- Updated: %s", orNA(task.Updated))

		if task.Due != "" {
			fmt.Fprintf(&sb, "\n- Due Date: %s", task.Due)
		}
		if task.Completed != nil && *task.Completed != "" {
			fmt.Fprintf(&sb, "\n- Completed: %s", *task.Completed)
		}
		if task.Notes != "" {
			fmt.Fprintf(&sb, "\n- Notes: %s", task.Notes)
		}
		if task.Parent != "" {
			fmt.Fprintf(&sb, "\n- Parent Task ID: %s", task.Parent)
		}
		if task.Position != "" {
			fmt.Fprintf(&sb, "\n- Position: %s", task.Position)
		}
		if task.SelfLink != "" {
			fmt.Fprintf(&sb, "\n- Self Link: %s", task.SelfLink)
		}
		if task.WebViewLink != "" {
			fmt.Fprintf(&sb, "\n- Web View Link: %s", task.WebViewLink)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_create_task ---

func registerCreateTask(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_create_task",
		mcp.WithDescription("Add a to-do (returns task_id). Use for 'remind me to…', new task. Prefer task_list_id=\"@default\". Not for edits — use tasks_update_task."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("Task list id. Use \"@default\" for the account default list.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Task title.")),
		mcp.WithString("notes", mcp.Description("Optional notes.")),
		mcp.WithString("due", mcp.Description("Optional due date RFC3339 (e.g. 2026-07-31T00:00:00Z).")),
	)
	s.AddTool(tool, handleCreateTask(getClient))
}

func handleCreateTask(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `tasks_create_task(task_list_id="@default", title=…)`), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return needArg("task_list_id", `tasks_create_task(task_list_id="@default", title=…)`), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return needArg("title", `tasks_create_task(task_list_id="@default", title=…)`), nil
		}

		notes := request.GetString("notes", "")
		due := request.GetString("due", "")
		parent := request.GetString("parent", "")
		previous := request.GetString("previous", "")

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := &tasks.Task{Title: title}
		if notes != "" {
			body.Notes = notes
		}
		if due != "" {
			body.Due = due
		}

		call := svc.Tasks.Insert(taskListID, body)
		if parent != "" {
			call = call.Parent(parent)
		}
		if previous != "" {
			call = call.Previous(previous)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating task: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Task Created for %s:\n", email)
		fmt.Fprintf(&sb, "- Title: %s\n", result.Title)
		fmt.Fprintf(&sb, "- ID: %s\n", result.Id)
		fmt.Fprintf(&sb, "- Status: %s\n", orNA(result.Status))
		fmt.Fprintf(&sb, "- Updated: %s", orNA(result.Updated))

		if result.Due != "" {
			fmt.Fprintf(&sb, "\n- Due Date: %s", result.Due)
		}
		if result.Notes != "" {
			fmt.Fprintf(&sb, "\n- Notes: %s", result.Notes)
		}
		if result.WebViewLink != "" {
			fmt.Fprintf(&sb, "\n- Web View Link: %s", result.WebViewLink)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_update_task ---

func registerUpdateTask(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_update_task",
		mcp.WithDescription("Update a task (title, notes, due, status). Required task_list_id + task_id from tasks_list_tasks. Mark done: status=\"completed\". Not for new tasks — use tasks_create_task."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("Task list id. Use \"@default\" for the account default list.")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task id from tasks_list_tasks.")),
		mcp.WithString("title", mcp.Description("New title.")),
		mcp.WithString("notes", mcp.Description("New notes.")),
		mcp.WithString("status", mcp.Description("needsAction or completed.")),
		mcp.WithString("due", mcp.Description("New due date RFC3339.")),
	)
	s.AddTool(tool, handleUpdateTask(getClient))
}

func handleUpdateTask(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `tasks_update_task(task_list_id="@default", task_id=…, …)`), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return needArg("task_list_id", `tasks_list_tasks(task_list_id="@default") then tasks_update_task(task_list_id, task_id, …)`), nil
		}
		taskID, err := request.RequireString("task_id")
		if err != nil {
			return needArg("task_id", `tasks_list_tasks(task_list_id="@default") then tasks_update_task(task_list_id, task_id, …)`), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch current task to preserve existing fields.
		current, err := svc.Tasks.Get(taskListID, taskID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting current task: %v", err)), nil
		}

		args := request.GetArguments()

		body := &tasks.Task{
			Id:     taskID,
			Title:  current.Title,
			Status: current.Status,
		}

		if _, ok := args["title"]; ok {
			body.Title = request.GetString("title", "")
		}
		if _, ok := args["status"]; ok {
			body.Status = request.GetString("status", "needsAction")
		}
		if v, ok := args["notes"]; ok {
			body.Notes, _ = v.(string)
		} else if current.Notes != "" {
			body.Notes = current.Notes
		}
		if v, ok := args["due"]; ok {
			body.Due, _ = v.(string)
		} else if current.Due != "" {
			body.Due = current.Due
		}

		result, err := svc.Tasks.Update(taskListID, taskID, body).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating task: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Task Updated for %s:\n", email)
		fmt.Fprintf(&sb, "- Title: %s\n", result.Title)
		fmt.Fprintf(&sb, "- ID: %s\n", result.Id)
		fmt.Fprintf(&sb, "- Status: %s\n", orNA(result.Status))
		fmt.Fprintf(&sb, "- Updated: %s", orNA(result.Updated))

		if result.Due != "" {
			fmt.Fprintf(&sb, "\n- Due Date: %s", result.Due)
		}
		if result.Notes != "" {
			fmt.Fprintf(&sb, "\n- Notes: %s", result.Notes)
		}
		if result.Completed != nil && *result.Completed != "" {
			fmt.Fprintf(&sb, "\n- Completed: %s", *result.Completed)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_delete_task ---

func registerDeleteTask(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_delete_task",
		mcp.WithDescription("Delete one task by task_list_id + task_id. Destructive — confirm when unclear. Prefer mark completed via tasks_update_task when unsure."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("Task list id. Use \"@default\" for the account default list.")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task id from tasks_list_tasks.")),
	)
	s.AddTool(tool, handleDeleteTask(getClient))
}

func handleDeleteTask(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `tasks_delete_task(task_list_id="@default", task_id=…)`), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return needArg("task_list_id", `tasks_list_tasks(task_list_id="@default") then tasks_delete_task(task_list_id, task_id)`), nil
		}
		taskID, err := request.RequireString("task_id")
		if err != nil {
			return needArg("task_id", `tasks_list_tasks(task_list_id="@default") then tasks_delete_task(task_list_id, task_id)`), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = svc.Tasks.Delete(taskListID, taskID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting task: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Task %s has been deleted from task list %s for %s.",
			taskID, taskListID, email,
		)), nil
	}
}

// --- tasks_move_task ---

func registerMoveTask(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_move_task",
		mcp.WithDescription("Reorder a task or move it to another tasklist_id / parent task. Required tasklist_id + task_id. Use after tasks_list_tasks when reorganizing."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("The ID of the current task list containing the task.")),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("The ID of the task to move.")),
		mcp.WithString("parent", mcp.Description("New parent task ID (for making it a subtask).")),
		mcp.WithString("previous", mcp.Description("Previous sibling task ID (for positioning).")),
		mcp.WithString("destination_task_list", mcp.Description("Destination task list ID (for moving between lists).")),
	)
	s.AddTool(tool, handleMoveTask(getClient))
}

func handleMoveTask(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskID, err := request.RequireString("task_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		parent := request.GetString("parent", "")
		previous := request.GetString("previous", "")
		destinationTaskList := request.GetString("destination_task_list", "")

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		call := svc.Tasks.Move(taskListID, taskID)
		if parent != "" {
			call = call.Parent(parent)
		}
		if previous != "" {
			call = call.Previous(previous)
		}
		if destinationTaskList != "" {
			call = call.DestinationTasklist(destinationTaskList)
		}

		result, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("moving task: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Task Moved for %s:\n", email)
		fmt.Fprintf(&sb, "- Title: %s\n", result.Title)
		fmt.Fprintf(&sb, "- ID: %s\n", result.Id)
		fmt.Fprintf(&sb, "- Status: %s\n", orNA(result.Status))
		fmt.Fprintf(&sb, "- Updated: %s", orNA(result.Updated))

		if result.Parent != "" {
			fmt.Fprintf(&sb, "\n- Parent Task ID: %s", result.Parent)
		}
		if result.Position != "" {
			fmt.Fprintf(&sb, "\n- Position: %s", result.Position)
		}

		var moveDetails []string
		if destinationTaskList != "" {
			moveDetails = append(moveDetails, "moved to task list "+destinationTaskList)
		}
		if parent != "" {
			moveDetails = append(moveDetails, "made a subtask of "+parent)
		}
		if previous != "" {
			moveDetails = append(moveDetails, "positioned after "+previous)
		}
		if len(moveDetails) > 0 {
			fmt.Fprintf(&sb, "\n- Move Details: %s", strings.Join(moveDetails, ", "))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- tasks_clear_completed ---

func registerClearCompletedTasks(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("tasks_clear_completed",
		mcp.WithDescription("Clear/hide all completed tasks in a tasklist_id. Required tasklist_id. Confirm when unclear — removes completed items from the active view."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("task_list_id", mcp.Required(), mcp.Description("The ID of the task list to clear completed tasks from.")),
	)
	s.AddTool(tool, handleClearCompletedTasks(getClient))
}

func handleClearCompletedTasks(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		taskListID, err := request.RequireString("task_list_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newTasksService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = svc.Tasks.Clear(taskListID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("clearing completed tasks: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"All completed tasks have been cleared from task list %s for %s. The tasks are now hidden and won't appear in default task list views.",
			taskListID, email,
		)), nil
	}
}
