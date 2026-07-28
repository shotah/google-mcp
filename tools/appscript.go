package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	drive "google.golang.org/api/drive/v3"
	script "google.golang.org/api/script/v1"

	google "github.com/shotah/google-workspace-mcp-go/internal/google"
	"github.com/shotah/google-workspace-mcp-go/server"
)

// RegisterAppScriptTools registers all Apps Script tools with the MCP server.
func RegisterAppScriptTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())
	registerListScriptProjects(s, getClient)
	registerGetScriptProject(s, getClient)
	registerGetScriptContent(s, getClient)
	registerListDeployments(s, getClient)
	registerListScriptProcesses(s, getClient)
	registerListVersions(s, getClient)
	registerGetVersion(s, getClient)
	registerGetScriptMetrics(s, getClient)
	registerCreateScriptProject(s, getClient)
	registerUpdateScriptContent(s, getClient)
	registerRunScriptFunction(s, getClient)
	registerCreateDeployment(s, getClient)
	registerUpdateDeployment(s, getClient)
	registerDeleteDeployment(s, getClient)
	registerDeleteScriptProject(s, getClient)
	registerCreateVersion(s, getClient)
	registerGenerateTriggerCode(s)
}

// newScriptService creates an authenticated script.Service for the given user.
func newScriptService(ctx context.Context, getClient httpClientFunc, email string) (*script.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for Apps Script: %w", err)
	}
	svc, err := script.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Apps Script service: %w", err)
	}
	return svc, nil
}

// newDriveServiceForScript creates an authenticated drive.Service for script-related operations.
func newDriveServiceForScript(ctx context.Context, getClient httpClientFunc, email string) (*drive.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for Drive (Apps Script): %w", err)
	}
	svc, err := drive.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Drive service for Apps Script: %w", err)
	}
	return svc, nil
}

// --- list_script_projects ---

func registerListScriptProjects(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_script_projects",
		mcp.WithDescription("List Apps Script projects (script_id, title). Use for 'my scripts' before get_script_project. Not for running code — use run_script_function."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of results per page (default: 50)."),
		),
		mcp.WithString("page_token",
			mcp.Description("Token for pagination."),
		),
	)
	RegisterTool(s, tool, makeListScriptProjectsHandler(getClient))
}

func makeListScriptProjectsHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageSize := request.GetInt("page_size", 50)
		pageToken := request.GetString("page_token", "")

		svc, err := newDriveServiceForScript(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		query := "mimeType='application/vnd.google-apps.script' and trashed=false"
		call := svc.Files.List().
			Q(query).
			PageSize(int64(pageSize)).
			Fields("nextPageToken, files(id, name, createdTime, modifiedTime)").
			OrderBy("modifiedTime desc").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing script projects: %v", err)), nil
		}

		if len(resp.Files) == 0 {
			return mcp.NewToolResultText("No Apps Script projects found."), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d Apps Script projects:\n", len(resp.Files))
		for _, f := range resp.Files {
			title := f.Name
			if title == "" {
				title = "Untitled"
			}
			scriptID := f.Id
			if scriptID == "" {
				scriptID = "Unknown ID"
			}
			createTime := f.CreatedTime
			if createTime == "" {
				createTime = "Unknown"
			}
			updateTime := f.ModifiedTime
			if updateTime == "" {
				updateTime = "Unknown"
			}
			fmt.Fprintf(&sb, "- %s (ID: %s) Created: %s Modified: %s\n", title, scriptID, createTime, updateTime)
		}

		if resp.NextPageToken != "" {
			fmt.Fprintf(&sb, "\nNext page token: %s", resp.NextPageToken)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- get_script_project ---

func registerGetScriptProject(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_script_project",
		mcp.WithDescription("Full project metadata + all source files for script_id. Use before update_script_content or run_script_function. Prefer get_script_content for one file."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
	)
	RegisterTool(s, tool, makeGetScriptProjectHandler(getClient))
}

func makeGetScriptProjectHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		project, err := svc.Projects.Get(scriptID).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting script project: %v", err)), nil
		}

		title := project.Title
		if title == "" {
			title = "Untitled"
		}
		projectScriptID := project.ScriptId
		if projectScriptID == "" {
			projectScriptID = "Unknown"
		}
		creator := "Unknown"
		if project.Creator != nil && project.Creator.Email != "" {
			creator = project.Creator.Email
		}
		createTime := project.CreateTime
		if createTime == "" {
			createTime = "Unknown"
		}
		updateTime := project.UpdateTime
		if updateTime == "" {
			updateTime = "Unknown"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Project: %s (ID: %s)\n", title, projectScriptID)
		fmt.Fprintf(&sb, "Creator: %s\n", creator)
		fmt.Fprintf(&sb, "Created: %s\n", createTime)
		fmt.Fprintf(&sb, "Modified: %s\n", updateTime)

		// Get project content (files) via GetContent
		content, err := svc.Projects.GetContent(scriptID).Context(ctx).Do()
		if err == nil && content != nil && len(content.Files) > 0 {
			sb.WriteString(formatScriptFiles(content.Files))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func formatScriptFiles(files []*script.File) string {
	var sb strings.Builder
	sb.WriteString("\nFiles:\n")
	for i, f := range files {
		fileName := f.Name
		if fileName == "" {
			fileName = "Untitled"
		}
		fileType := f.Type
		if fileType == "" {
			fileType = "Unknown"
		}
		fmt.Fprintf(&sb, "%d. %s (%s)\n", i+1, fileName, fileType)
		if f.Source != "" {
			source := f.Source
			if len(source) > 200 {
				source = source[:200] + "..."
			}
			fmt.Fprintf(&sb, "   %s\n\n", source)
		}
	}
	return sb.String()
}

// --- get_script_content ---

func registerGetScriptContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_script_content",
		mcp.WithDescription("Read one source file in script_id by file name. Use when you only need a single .gs file. Full project: get_script_project."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("file_name",
			mcp.Required(),
			mcp.Description("Name of the file to retrieve."),
		),
	)
	RegisterTool(s, tool, makeGetScriptContentHandler(getClient))
}

func makeGetScriptContentHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fileName, err := request.RequireString("file_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := svc.Projects.GetContent(scriptID).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting script content: %v", err)), nil
		}

		for _, f := range content.Files {
			if f.Name == fileName {
				fileType := f.Type
				if fileType == "" {
					fileType = "Unknown"
				}
				var sb strings.Builder
				fmt.Fprintf(&sb, "File: %s (%s)\n\n%s", fileName, fileType, f.Source)
				return mcp.NewToolResultText(sb.String()), nil
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("File '%s' not found in project %s", fileName, scriptID)), nil
	}
}

// --- list_deployments ---

func registerListDeployments(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_deployments",
		mcp.WithDescription("List deployments for script_id (deployment ids, entry points). Use before run_script_function or update_deployment."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
	)
	RegisterTool(s, tool, makeListDeploymentsHandler(getClient))
}

func makeListDeploymentsHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.Projects.Deployments.List(scriptID).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing deployments: %v", err)), nil
		}

		if len(resp.Deployments) == 0 {
			return mcp.NewToolResultText("No deployments found for script: " + scriptID), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Deployments for script: %s\n\n", scriptID)
		for i, d := range resp.Deployments {
			deploymentID := d.DeploymentId
			if deploymentID == "" {
				deploymentID = "Unknown"
			}
			description := "No description"
			if d.DeploymentConfig != nil && d.DeploymentConfig.Description != "" {
				description = d.DeploymentConfig.Description
			}
			updateTime := d.UpdateTime
			if updateTime == "" {
				updateTime = "Unknown"
			}
			fmt.Fprintf(&sb, "%d. %s (%s)\n", i+1, description, deploymentID)
			fmt.Fprintf(&sb, "   Updated: %s\n\n", updateTime)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_script_processes ---

func registerListScriptProcesses(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_script_processes",
		mcp.WithDescription("Recent script execution processes (status, errors). Use for 'why did my script fail'. Not project source — use get_script_project."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithNumber("page_size",
			mcp.Description("Number of results (default: 50)."),
		),
		mcp.WithString("script_id",
			mcp.Description("Optional filter by script ID."),
		),
	)
	RegisterTool(s, tool, makeListScriptProcessesHandler(getClient))
}

func makeListScriptProcessesHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pageSize := request.GetInt("page_size", 50)
		scriptID := request.GetString("script_id", "")

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		call := svc.Processes.List().PageSize(int64(pageSize))
		if scriptID != "" {
			call = call.UserProcessFilterScriptId(scriptID)
		}

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing script processes: %v", err)), nil
		}

		if len(resp.Processes) == 0 {
			return mcp.NewToolResultText("No recent script executions found."), nil
		}

		var sb strings.Builder
		sb.WriteString("Recent script executions:\n\n")
		for i, p := range resp.Processes {
			functionName := p.FunctionName
			if functionName == "" {
				functionName = "Unknown"
			}
			processStatus := p.ProcessStatus
			if processStatus == "" {
				processStatus = "Unknown"
			}
			startTime := p.StartTime
			if startTime == "" {
				startTime = "Unknown"
			}
			duration := p.Duration
			if duration == "" {
				duration = "Unknown"
			}
			fmt.Fprintf(&sb, "%d. %s\n", i+1, functionName)
			fmt.Fprintf(&sb, "   Status: %s\n", processStatus)
			fmt.Fprintf(&sb, "   Started: %s\n", startTime)
			fmt.Fprintf(&sb, "   Duration: %s\n\n", duration)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- list_versions ---

func registerListVersions(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("list_versions",
		mcp.WithDescription("List immutable versions for script_id. Use before get_version or create_deployment. Snapshots for rollback — create_version to add new."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
	)
	RegisterTool(s, tool, makeListVersionsHandler(getClient))
}

func makeListVersionsHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.Projects.Versions.List(scriptID).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing versions: %v", err)), nil
		}

		if len(resp.Versions) == 0 {
			return mcp.NewToolResultText("No versions found for script: " + scriptID), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Versions for script: %s\n\n", scriptID)
		for _, v := range resp.Versions {
			description := v.Description
			if description == "" {
				description = "No description"
			}
			createTime := v.CreateTime
			if createTime == "" {
				createTime = "Unknown"
			}
			fmt.Fprintf(&sb, "Version %d: %s\n", v.VersionNumber, description)
			fmt.Fprintf(&sb, "   Created: %s\n\n", createTime)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- get_version ---

func registerGetVersion(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_version",
		mcp.WithDescription("Get one script version by version number on script_id. Use after list_versions."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithNumber("version_number",
			mcp.Required(),
			mcp.Description("The version number to retrieve (1, 2, 3, etc.)."),
		),
	)
	RegisterTool(s, tool, makeGetVersionHandler(getClient))
}

func makeGetVersionHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		versionNumber := request.GetInt("version_number", 0)
		if versionNumber == 0 {
			return mcp.NewToolResultError("version_number is required"), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		version, err := svc.Projects.Versions.Get(scriptID, int64(versionNumber)).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting version: %v", err)), nil
		}

		description := version.Description
		if description == "" {
			description = "No description"
		}
		createTime := version.CreateTime
		if createTime == "" {
			createTime = "Unknown"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Version %d of script: %s\n", version.VersionNumber, scriptID)
		fmt.Fprintf(&sb, "Description: %s\n", description)
		fmt.Fprintf(&sb, "Created: %s", createTime)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- get_script_metrics ---

func registerGetScriptMetrics(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("get_script_metrics",
		mcp.WithDescription("Execution metrics for script_id (usage stats). Use for monitoring. Not live run — use run_script_function or list_script_processes."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("metrics_granularity",
			mcp.Description("Granularity of metrics — \"DAILY\" or \"WEEKLY\". Defaults to \"DAILY\"."),
			mcp.Enum("DAILY", "WEEKLY"),
		),
	)
	RegisterTool(s, tool, makeGetScriptMetricsHandler(getClient))
}

func makeGetScriptMetricsHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		granularity := request.GetString("metrics_granularity", "DAILY")

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		metrics, err := svc.Projects.GetMetrics(scriptID).
			MetricsGranularity(granularity).
			Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting script metrics: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Metrics for script: %s\n", scriptID)
		fmt.Fprintf(&sb, "Granularity: %s\n\n", granularity)

		hasData := false

		if len(metrics.ActiveUsers) > 0 {
			hasData = true
			sb.WriteString("Active Users:\n")
			for _, m := range metrics.ActiveUsers {
				startTime := m.StartTime
				if startTime == "" {
					startTime = "Unknown"
				}
				endTime := m.EndTime
				if endTime == "" {
					endTime = "Unknown"
				}
				fmt.Fprintf(&sb, "  %s to %s: %d users\n", startTime, endTime, m.Value)
			}
			sb.WriteString("\n")
		}

		if len(metrics.TotalExecutions) > 0 {
			hasData = true
			sb.WriteString("Total Executions:\n")
			for _, m := range metrics.TotalExecutions {
				startTime := m.StartTime
				if startTime == "" {
					startTime = "Unknown"
				}
				endTime := m.EndTime
				if endTime == "" {
					endTime = "Unknown"
				}
				fmt.Fprintf(&sb, "  %s to %s: %d executions\n", startTime, endTime, m.Value)
			}
			sb.WriteString("\n")
		}

		if len(metrics.FailedExecutions) > 0 {
			hasData = true
			sb.WriteString("Failed Executions:\n")
			for _, m := range metrics.FailedExecutions {
				startTime := m.StartTime
				if startTime == "" {
					startTime = "Unknown"
				}
				endTime := m.EndTime
				if endTime == "" {
					endTime = "Unknown"
				}
				fmt.Fprintf(&sb, "  %s to %s: %d failures\n", startTime, endTime, m.Value)
			}
			sb.WriteString("\n")
		}

		if !hasData {
			sb.WriteString("No metrics data available for this script.")
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- create_script_project ---

func registerCreateScriptProject(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("create_script_project",
		mcp.WithDescription("Create empty Apps Script project (title). Returns script_id. Use for 'new Apps Script'. Add code with update_script_content."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Project title."),
		),
		mcp.WithString("parent_id",
			mcp.Description("Optional Drive folder ID or bound container ID."),
		),
	)
	RegisterTool(s, tool, makeCreateScriptProjectHandler(getClient))
}

func makeCreateScriptProjectHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		parentID := request.GetString("parent_id", "")

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := &script.CreateProjectRequest{
			Title: title,
		}
		if parentID != "" {
			body.ParentId = parentID
		}

		project, err := svc.Projects.Create(body).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating script project: %v", err)), nil
		}

		scriptID := project.ScriptId
		if scriptID == "" {
			scriptID = "Unknown"
		}
		editURL := fmt.Sprintf("https://script.google.com/d/%s/edit", scriptID)

		var sb strings.Builder
		fmt.Fprintf(&sb, "Created Apps Script project: %s\n", title)
		fmt.Fprintf(&sb, "Script ID: %s\n", scriptID)
		fmt.Fprintf(&sb, "Edit URL: %s", editURL)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- update_script_content ---

func registerUpdateScriptContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("update_script_content",
		mcp.WithDescription("Create/update .gs files in script_id. Use for 'edit script source'. Deploy/run after saving. Not for execute — use run_script_function."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithArray("files",
			mcp.Required(),
			mcp.Description("List of file objects with name, type, and source."),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":   map[string]any{"type": "string"},
					"type":   map[string]any{"type": "string"},
					"source": map[string]any{"type": "string"},
				},
			}),
		),
	)
	RegisterTool(s, tool, makeUpdateScriptContentHandler(getClient))
}

func makeUpdateScriptContentHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := request.GetArguments()
		filesRaw, ok := args["files"]
		if !ok {
			return mcp.NewToolResultError("files is required"), nil
		}

		filesSlice, ok := filesRaw.([]any)
		if !ok {
			return mcp.NewToolResultError("files must be an array"), nil
		}

		var files []*script.File
		for _, item := range filesSlice {
			m, ok := item.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("each file must be an object with name, type, and source"), nil
			}
			f := &script.File{}
			if v, ok := m["name"].(string); ok {
				f.Name = v
			}
			if v, ok := m["type"].(string); ok {
				f.Type = v
			}
			if v, ok := m["source"].(string); ok {
				f.Source = v
			}
			files = append(files, f)
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content := &script.Content{
			Files: files,
		}

		updated, err := svc.Projects.UpdateContent(scriptID, content).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating script content: %v", err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Updated script project: %s\n\nModified files:\n", scriptID)
		for _, f := range updated.Files {
			fileName := f.Name
			if fileName == "" {
				fileName = "Untitled"
			}
			fileType := f.Type
			if fileType == "" {
				fileType = "Unknown"
			}
			fmt.Fprintf(&sb, "- %s (%s)\n", fileName, fileType)
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- run_script_function ---

func registerRunScriptFunction(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("run_script_function",
		mcp.WithDescription("Run a function in a deployed script (script_id, deployment, function name, params). Use for 'execute my script'. Confirm side effects. Needs list_deployments first."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("function_name",
			mcp.Required(),
			mcp.Description("Name of function to execute."),
		),
		mcp.WithArray("parameters",
			mcp.Description("Optional list of parameters to pass."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithBoolean("dev_mode",
			mcp.Description("Whether to run latest code vs deployed version."),
		),
	)
	RegisterTool(s, tool, makeRunScriptFunctionHandler(getClient))
}

func makeRunScriptFunctionHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		functionName, err := request.RequireString("function_name")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		devMode := getBool(request, "dev_mode", false)

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		execReq := &script.ExecutionRequest{
			Function: functionName,
			DevMode:  devMode,
		}

		// Extract optional parameters
		args := request.GetArguments()
		if paramsRaw, ok := args["parameters"]; ok {
			if paramsSlice, ok := paramsRaw.([]any); ok && len(paramsSlice) > 0 {
				execReq.Parameters = paramsSlice
			}
		}

		if devMode {
			execReq.ForceSendFields = append(execReq.ForceSendFields, "DevMode")
		}

		operation, err := svc.Scripts.Run(scriptID, execReq).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("Execution failed\nFunction: %s\nError: %v", functionName, err)), nil
		}

		if operation.Error != nil {
			errorMsg := "Unknown error"
			if operation.Error.Message != "" {
				errorMsg = operation.Error.Message
			}
			return mcp.NewToolResultText(fmt.Sprintf("Execution failed\nFunction: %s\nError: %s", functionName, errorMsg)), nil
		}

		// Parse the result from the response
		var result any
		if operation.Response != nil {
			// Response is a googleapi.RawMessage — unmarshal to get the result field
			var respMap map[string]any
			if err := json.Unmarshal(operation.Response, &respMap); err == nil {
				result = respMap["result"]
			}
		}

		var sb strings.Builder
		sb.WriteString("Execution successful\n")
		fmt.Fprintf(&sb, "Function: %s\n", functionName)
		fmt.Fprintf(&sb, "Result: %v", result)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- create_deployment ---

func registerCreateDeployment(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("create_deployment",
		mcp.WithDescription("Create new deployment of script_id (API executable, web app, etc.). Use after update_script_content. Not for run — use run_script_function."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Deployment description."),
		),
		mcp.WithString("version_description",
			mcp.Description("Optional version description."),
		),
	)
	RegisterTool(s, tool, makeCreateDeploymentHandler(getClient))
}

func makeCreateDeploymentHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		description, err := request.RequireString("description")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		versionDescription := request.GetString("version_description", "")
		if versionDescription == "" {
			versionDescription = description
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// First, create a new version
		versionBody := &script.Version{
			Description: versionDescription,
		}
		version, err := svc.Projects.Versions.Create(scriptID, versionBody).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating version for deployment: %v", err)), nil
		}

		// Now create the deployment with the version number
		deploymentConfig := &script.DeploymentConfig{
			VersionNumber: version.VersionNumber,
			Description:   description,
		}

		deployment, err := svc.Projects.Deployments.Create(scriptID, deploymentConfig).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating deployment: %v", err)), nil
		}

		deploymentID := deployment.DeploymentId
		if deploymentID == "" {
			deploymentID = "Unknown"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Created deployment for script: %s\n", scriptID)
		fmt.Fprintf(&sb, "Deployment ID: %s\n", deploymentID)
		fmt.Fprintf(&sb, "Version: %d\n", version.VersionNumber)
		fmt.Fprintf(&sb, "Description: %s", description)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- update_deployment ---

func registerUpdateDeployment(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("update_deployment",
		mcp.WithDescription("Update deployment config by deployment_id on script_id. Not for delete — use delete_deployment."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("deployment_id",
			mcp.Required(),
			mcp.Description("The deployment ID to update."),
		),
		mcp.WithString("description",
			mcp.Description("Optional new description."),
		),
	)
	RegisterTool(s, tool, makeUpdateDeploymentHandler(getClient))
}

func makeUpdateDeploymentHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		deploymentID, err := request.RequireString("deployment_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		description := request.GetString("description", "")

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		updateReq := &script.UpdateDeploymentRequest{
			DeploymentConfig: &script.DeploymentConfig{},
		}
		if description != "" {
			updateReq.DeploymentConfig.Description = description
		}

		deployment, err := svc.Projects.Deployments.Update(scriptID, deploymentID, updateReq).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("updating deployment: %v", err)), nil
		}

		respDescription := "No description"
		if deployment.DeploymentConfig != nil && deployment.DeploymentConfig.Description != "" {
			respDescription = deployment.DeploymentConfig.Description
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Updated deployment: %s\n", deploymentID)
		fmt.Fprintf(&sb, "Script: %s\n", scriptID)
		fmt.Fprintf(&sb, "Description: %s", respDescription)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- delete_deployment ---

func registerDeleteDeployment(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("delete_deployment",
		mcp.WithDescription("Delete deployment by deployment_id on script_id. Destructive — confirm when unclear."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("deployment_id",
			mcp.Required(),
			mcp.Description("The deployment ID to delete."),
		),
	)
	RegisterTool(s, tool, makeDeleteDeploymentHandler(getClient))
}

func makeDeleteDeploymentHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		deploymentID, err := request.RequireString("deployment_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		_, err = svc.Projects.Deployments.Delete(scriptID, deploymentID).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting deployment: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Deleted deployment: %s from script: %s", deploymentID, scriptID)), nil
	}
}

// --- delete_script_project ---

func registerDeleteScriptProject(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("delete_script_project",
		mcp.WithDescription("Permanently delete script_id and all files. Cannot undo — confirm always. Not for undeploy only — use delete_deployment."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID to delete."),
		),
	)
	RegisterTool(s, tool, makeDeleteScriptProjectHandler(getClient))
}

func makeDeleteScriptProjectHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Apps Script projects are stored as Drive files — use Drive API to delete
		svc, err := newDriveServiceForScript(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = svc.Files.Delete(scriptID).SupportsAllDrives(true).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("deleting script project: %v", err)), nil
		}

		return mcp.NewToolResultText("Deleted Apps Script project: " + scriptID), nil
	}
}

// --- create_version ---

func registerCreateVersion(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("create_version",
		mcp.WithDescription("Snapshot current script code as immutable version on script_id. Use before risky deploys. Versions cannot be edited — create_version again for new snapshot."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("script_id",
			mcp.Required(),
			mcp.Description("The script project ID."),
		),
		mcp.WithString("description",
			mcp.Description("Optional description for this version."),
		),
	)
	RegisterTool(s, tool, makeCreateVersionHandler(getClient))
}

func makeCreateVersionHandler(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		scriptID, err := request.RequireString("script_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		description := request.GetString("description", "")

		svc, err := newScriptService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := &script.Version{}
		if description != "" {
			body.Description = description
		}

		version, err := svc.Projects.Versions.Create(scriptID, body).Context(ctx).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating version: %v", err)), nil
		}

		createTime := version.CreateTime
		if createTime == "" {
			createTime = "Unknown"
		}
		descriptionText := description
		if descriptionText == "" {
			descriptionText = "No description"
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Created version %d for script: %s\n", version.VersionNumber, scriptID)
		fmt.Fprintf(&sb, "Description: %s\n", descriptionText)
		fmt.Fprintf(&sb, "Created: %s", createTime)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// --- generate_trigger_code ---

func registerGenerateTriggerCode(s *mcpserver.MCPServer) {
	tool := mcp.NewTool("generate_trigger_code",
		mcp.WithDescription("Generate Apps Script snippet to create time/event triggers (API cannot add triggers directly). Use for 'schedule this function'. User pastes code into the script editor."),
		mcp.WithString("trigger_type",
			mcp.Required(),
			mcp.Description("Type of trigger: time_minutes, time_hours, time_daily, time_weekly, on_open, on_edit, on_form_submit, on_change."),
			mcp.Enum("time_minutes", "time_hours", "time_daily", "time_weekly", "on_open", "on_edit", "on_form_submit", "on_change"),
		),
		mcp.WithString("function_name",
			mcp.Required(),
			mcp.Description("The function to run when trigger fires (e.g., \"sendDailyReport\")."),
		),
		mcp.WithString("schedule",
			mcp.Description("Schedule details (depends on trigger_type): For time_minutes: \"1\", \"5\", \"10\", \"15\", or \"30\". For time_hours: \"1\", \"2\", \"4\", \"6\", \"8\", or \"12\". For time_daily: hour as \"0\"-\"23\". For time_weekly: \"MONDAY\", \"TUESDAY\", etc. For simple triggers: not needed."),
		),
	)
	RegisterTool(s, tool, generateTriggerCodeHandler)
}

func generateTriggerCodeHandler(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	triggerType, err := request.RequireString("trigger_type")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	functionName, err := request.RequireString("function_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	schedule := request.GetString("schedule", "")

	var codeLines []string
	var instructions []string

	switch triggerType {
	case "on_open":
		codeLines = []string{
			"// Simple trigger - just rename your function to 'onOpen'",
			"// This runs automatically when the document is opened",
			"function onOpen(e) {",
			fmt.Sprintf("  %s();", functionName),
			"}",
		}
	case "on_edit":
		codeLines = []string{
			"// Simple trigger - just rename your function to 'onEdit'",
			"// This runs automatically when a user edits the spreadsheet",
			"function onEdit(e) {",
			fmt.Sprintf("  %s();", functionName),
			"}",
		}
	case "time_minutes":
		interval := schedule
		if interval == "" {
			interval = "5"
		}
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			fmt.Sprintf("function createTimeTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			fmt.Sprintf("  // Create new trigger - runs every %s minutes", interval),
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .timeBased()",
			fmt.Sprintf("    .everyMinutes(%s)", interval),
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run every %s minutes');", functionName, interval),
			"}",
		}
	case "time_hours":
		interval := schedule
		if interval == "" {
			interval = "1"
		}
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			fmt.Sprintf("function createTimeTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			fmt.Sprintf("  // Create new trigger - runs every %s hour(s)", interval),
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .timeBased()",
			fmt.Sprintf("    .everyHours(%s)", interval),
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run every %s hour(s)');", functionName, interval),
			"}",
		}
	case "time_daily":
		hour := schedule
		if hour == "" {
			hour = "9"
		}
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			fmt.Sprintf("function createDailyTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			fmt.Sprintf("  // Create new trigger - runs daily at %s:00", hour),
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .timeBased()",
			fmt.Sprintf("    .atHour(%s)", hour),
			"    .everyDays(1)",
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run daily at %s:00');", functionName, hour),
			"}",
		}
	case "time_weekly":
		day := strings.ToUpper(schedule)
		if day == "" {
			day = "MONDAY"
		}
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			fmt.Sprintf("function createWeeklyTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			"  // Create new trigger - runs weekly on " + day,
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .timeBased()",
			fmt.Sprintf("    .onWeekDay(ScriptApp.WeekDay.%s)", day),
			"    .atHour(9)",
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run every %s at 9:00');", functionName, day),
			"}",
		}
	case "on_form_submit":
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			"// This must be run from a script BOUND to the Google Form",
			fmt.Sprintf("function createFormSubmitTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			"  // Create new trigger - runs when form is submitted",
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .forForm(FormApp.getActiveForm())",
			"    .onFormSubmit()",
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run on form submit');", functionName),
			"}",
		}
	case "on_change":
		codeLines = []string{
			"// Run this function ONCE to install the trigger",
			"// This must be run from a script BOUND to a Google Sheet",
			fmt.Sprintf("function createChangeTrigger_%s() {", functionName),
			"  // Delete existing triggers for this function first",
			"  const triggers = ScriptApp.getProjectTriggers();",
			"  triggers.forEach(trigger => {",
			fmt.Sprintf("    if (trigger.getHandlerFunction() === '%s') {", functionName),
			"      ScriptApp.deleteTrigger(trigger);",
			"    }",
			"  });",
			"",
			"  // Create new trigger - runs when spreadsheet changes",
			fmt.Sprintf("  ScriptApp.newTrigger('%s')", functionName),
			"    .forSpreadsheet(SpreadsheetApp.getActive())",
			"    .onChange()",
			"    .create();",
			"",
			fmt.Sprintf("  Logger.log('Trigger created: %s will run on spreadsheet change');", functionName),
			"}",
		}
	default:
		return mcp.NewToolResultText(fmt.Sprintf("Unknown trigger type: %s\n\nValid types: time_minutes, time_hours, time_daily, time_weekly, on_open, on_edit, on_form_submit, on_change", triggerType)), nil
	}

	code := strings.Join(codeLines, "\n")

	if strings.HasPrefix(triggerType, "on_") {
		if triggerType == "on_open" || triggerType == "on_edit" {
			instructions = []string{
				"SIMPLE TRIGGER",
				strings.Repeat("=", 50),
				"",
				"Add this code to your script. Simple triggers run automatically",
				"when the event occurs - no setup function needed.",
				"",
				"Note: Simple triggers have limitations:",
				"- Cannot access services that require authorization",
				"- Cannot run longer than 30 seconds",
				"- Cannot make external HTTP requests",
				"",
				"For more capabilities, use an installable trigger instead.",
				"",
				"CODE TO ADD:",
				strings.Repeat("-", 50),
			}
		} else {
			instructions = []string{
				"INSTALLABLE TRIGGER",
				strings.Repeat("=", 50),
				"",
				"1. Add this code to your script",
				fmt.Sprintf("2. Run the setup function once: createFormSubmitTrigger_%s() or similar", functionName),
				"3. The trigger will then run automatically",
				"",
				"CODE TO ADD:",
				strings.Repeat("-", 50),
			}
		}
	} else {
		instructions = []string{
			"INSTALLABLE TRIGGER",
			strings.Repeat("=", 50),
			"",
			"1. Add this code to your script using update_script_content",
			"2. Run the setup function ONCE (manually in Apps Script editor or via run_script_function)",
			"3. The trigger will then run automatically on schedule",
			"",
			"To check installed triggers: Apps Script editor > Triggers (clock icon)",
			"",
			"CODE TO ADD:",
			strings.Repeat("-", 50),
		}
	}

	return mcp.NewToolResultText(strings.Join(instructions, "\n") + "\n\n" + code), nil
}
