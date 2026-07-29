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
