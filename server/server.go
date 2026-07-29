// Package server initializes and configures the MCP server.
package server

import (
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const ServerName = "google"

// ServerVersion is overwritten at link time by GoReleaser / `make cli`
// (-X github.com/shotah/google-mcp/server.ServerVersion=...).
var ServerVersion = "0.1.0"

// Config holds server configuration from CLI flags.
type Config struct {
	Tools    []string
	ToolTier string
	// Capability is read|edit|complete. Empty means complete (no capability filter).
	Capability string
	Transport  string
	SingleUser bool
	ReadOnly   bool
}

// New creates a new MCP server with the given configuration.
func New(cfg Config) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		ServerName,
		ServerVersion,
		mcpserver.WithToolCapabilities(true),
	)
	return s
}
