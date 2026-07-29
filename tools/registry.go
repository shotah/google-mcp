// Package tools provides tool registration and handlers for all Google Workspace services.
package tools

import (
	"errors"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/auth"
	"github.com/shotah/google-mcp/server"
)

// RegisterTool is a helper that wraps mcp.NewTool + server.AddTool.
func RegisterTool(s *mcpserver.MCPServer, tool mcp.Tool, handler mcpserver.ToolHandlerFunc) {
	s.AddTool(tool, handler)
}

// resolveEmail extracts the user's Google email from the request parameters,
// falling back to the USER_GOOGLE_EMAIL environment variable, and finally
// to the single available credential in the credential store.
func resolveEmail(request mcp.CallToolRequest) (string, error) {
	// 1. Try the explicit request parameter.
	email, _ := request.RequireString("user_google_email")
	if strings.TrimSpace(email) != "" {
		return email, nil
	}

	// 2. Fall back to USER_GOOGLE_EMAIL env var.
	if envEmail := os.Getenv("USER_GOOGLE_EMAIL"); strings.TrimSpace(envEmail) != "" {
		return envEmail, nil
	}

	// 3. Fall back to the single credential in the store (single-user mode).
	store := auth.NewCredentialStore()
	users, err := store.ListUsers()
	if err == nil && len(users) == 1 {
		return users[0], nil
	}

	return "", errors.New("user_google_email is required: provide it as a parameter, set USER_GOOGLE_EMAIL env var, or use --single-user with one credential file")
}

// RegisterAllTools registers tools for all enabled services based on config.
// If cfg.Tools is empty, all services are registered.
func RegisterAllTools(s *mcpserver.MCPServer, cfg server.Config) {
	enabled := make(map[string]bool, len(cfg.Tools))
	for _, t := range cfg.Tools {
		enabled[t] = true
	}
	allTools := len(cfg.Tools) == 0

	type registrar struct {
		name string
		fn   func(*mcpserver.MCPServer, server.Config)
	}

	registrars := []registrar{
		{"gmail", RegisterGmailTools},
		{"drive", RegisterDriveTools},
		{"calendar", RegisterCalendarTools},
		{"docs", RegisterDocsTools},
		{"sheets", RegisterSheetsTools},
		{"chat", RegisterChatTools},
		{"forms", RegisterFormsTools},
		{"slides", RegisterSlidesTools},
		{"tasks", RegisterTasksTools},
		{"contacts", RegisterContactsTools},
		{"search", RegisterSearchTools},
		{"appscript", RegisterAppScriptTools},
	}

	for _, r := range registrars {
		if allTools || enabled[r.name] {
			r.fn(s, cfg)
		}
	}

	// Register meta-tools (not tied to a specific service).
	// auth_start is under gmail/complete in tier config but is
	// registered independently. It loads when gmail is enabled or all tools are loaded.
	if allTools || enabled["gmail"] {
		RegisterAuthTools(s)
	}
}
