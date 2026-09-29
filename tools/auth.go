package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/auth"
	google "github.com/shotah/google-mcp/internal/google"
)

// RegisterListAccountsTool registers auth_list_accounts.
// It returns email addresses only, never token material.
func RegisterListAccountsTool(s *mcpserver.MCPServer) {
	store := auth.NewCredentialStore()
	s.AddTool(
		newMCPTool("auth_list_accounts",
			mcp.WithDescription("List Google accounts signed in on this host (email addresses only). Use to choose an account, then pass that address as user_google_email. Not for finding people — use contacts_search."),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			users, err := store.ListUsers()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("listing accounts: %v", err)), nil
			}
			if len(users) == 0 {
				return mcp.NewToolResultText("No Google accounts are signed in. A person runs `google-mcp auth` once per account."), nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Signed-in Google accounts (%d):", len(users))
			for _, email := range users {
				fmt.Fprintf(&b, "\n- %s", email)
			}
			b.WriteString("\nPass one of these as user_google_email on later calls.")
			return mcp.NewToolResultText(b.String()), nil
		},
	)
}

// RegisterAuthTools registers the auth_start meta-tool.
// It is filtered out when MCP_ENABLE_OAUTH21=true.
func RegisterAuthTools(s *mcpserver.MCPServer) {
	if isOAuth21Enabled() {
		return
	}

	store := auth.NewCredentialStore()

	s.AddTool(
		newMCPTool("auth_start",
			mcp.WithDescription(
				"Rare re-auth escape hatch. Prefer human CLI setup: `google-mcp auth` (or login) once; MCP tools refresh tokens automatically. Returns an auth URL if re-auth is needed. Disabled when MCP_ENABLE_OAUTH21=true. Requires service_name (e.g. Gmail).",
			),
			mcp.WithString("service_name",
				mcp.Required(),
				mcp.Description("Name of the Google service requiring authentication (e.g., 'Google Calendar', 'Gmail')"),
			),
			mcp.WithString("user_google_email",
				mcp.Description("User's Google email address for authentication"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			serviceName, err := request.RequireString("service_name")
			if err != nil {
				return mcp.NewToolResultError("service_name is required"), nil
			}

			userEmail, emailErr := resolveEmail(request)
			if emailErr != nil {
				return mcp.NewToolResultError(emailErr.Error()), nil
			}

			msg, err := auth.StartAuthFlow(ctx, serviceName, userEmail, store, func(email string) {
				// Invalidate cached client so next tool call picks up new credentials.
				google.DefaultClientCache().Invalidate(email)
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("**Authentication Error:** %v", err)), nil
			}

			return mcp.NewToolResultText(msg), nil
		},
	)
}

// isOAuth21Enabled checks if the MCP_ENABLE_OAUTH21 env var is set to "true".
func isOAuth21Enabled() bool {
	return strings.ToLower(os.Getenv("MCP_ENABLE_OAUTH21")) == "true"
}
