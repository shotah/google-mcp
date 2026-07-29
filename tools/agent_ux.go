package tools

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// newMCPTool builds a tool with MCP readOnly / destructive hints from tier metadata.
// Prefer short descriptions and lean schemas; teach the next call in error text instead.
func newMCPTool(name string, opts ...mcp.ToolOption) mcp.Tool {
	if readOnlyTools[name] {
		opts = append(opts, mcp.WithReadOnlyHintAnnotation(true))
	} else if destructiveTools[name] || isDestructiveToolName(name) {
		opts = append(opts, mcp.WithDestructiveHintAnnotation(true))
	}
	return mcp.NewTool(name, opts...)
}

func isDestructiveToolName(name string) bool {
	switch {
	case strings.Contains(name, "_delete_"),
		strings.Contains(name, "_clear_"),
		strings.HasSuffix(name, "_delete"),
		strings.Contains(name, "transfer_ownership"),
		strings.Contains(name, "batch_delete"),
		strings.Contains(name, "_resolve_comment"):
		return true
	default:
		return false
	}
}

// needArg returns a tool error that names the missing arg and the exact next call shape.
func needArg(arg, nextCall string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s is required. Next: %s", arg, nextCall))
}

// toolHint returns a tool error with a concrete next-call instruction.
func toolHint(msg, nextCall string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s. Next: %s", msg, nextCall))
}
