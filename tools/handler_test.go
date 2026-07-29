package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/server"
)

// newToolTestServer creates an MCP server with all tools registered, a temp
// credential dir (empty, so no single-user fallback), and a fake email env var
// so that resolveEmail succeeds.
func newToolTestServer(t *testing.T) *mcpserver.MCPServer {
	t.Helper()
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())
	t.Setenv("USER_GOOGLE_EMAIL", "test@example.com")
	cfg := server.Config{}
	s := server.New(cfg)
	RegisterAllTools(s, cfg)
	FilterTools(s, cfg)
	return s
}

// callToolRaw sends a tools/call JSON-RPC message and returns the raw
// response, which is either a JSONRPCResponse or JSONRPCError.
func callToolRaw(t *testing.T, s *mcpserver.MCPServer, toolName string, args map[string]any) mcp.JSONRPCMessage {
	t.Helper()
	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  params,
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal tools/call: %v", err)
	}
	return s.HandleMessage(context.Background(), raw)
}

// callTool sends a tools/call JSON-RPC message and returns the text content
// and isError flag. It fails the test if the response is a protocol-level
// error (JSONRPCError) rather than a tool-level result.
func callTool(t *testing.T, s *mcpserver.MCPServer, toolName string, args map[string]any) (text string, isError bool) {
	t.Helper()
	resp := callToolRaw(t, s, toolName, args)
	switch r := resp.(type) {
	case mcp.JSONRPCResponse:
		// mcp-go v0.57+ returns *CallToolResult from the server.
		result, ok := r.Result.(*mcp.CallToolResult)
		if !ok {
			t.Fatalf("expected *CallToolResult, got %T", r.Result)
		}
		if len(result.Content) == 0 {
			return "", result.IsError
		}
		tc, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("expected TextContent, got %T", result.Content[0])
		}
		return tc.Text, result.IsError
	case mcp.JSONRPCError:
		t.Fatalf("protocol error %d: %s", r.Error.Code, r.Error.Message)
		return "", true
	default:
		t.Fatalf("unexpected response type %T", resp)
		return "", true
	}
}

// --- Smoke tests ---

func TestHandlerNonexistentTool(t *testing.T) {
	s := newToolTestServer(t)
	resp := callToolRaw(t, s, "this_tool_does_not_exist", nil)
	rpcErr, ok := resp.(mcp.JSONRPCError)
	if !ok {
		t.Fatalf("expected JSONRPCError for nonexistent tool, got %T", resp)
	}
	if rpcErr.Error.Code != mcp.INVALID_PARAMS {
		t.Errorf("expected INVALID_PARAMS code, got %d", rpcErr.Error.Code)
	}
	if !strings.Contains(rpcErr.Error.Message, "not found") {
		t.Errorf("expected 'not found' in error, got %q", rpcErr.Error.Message)
	}
}

func TestHandlerMissingRequiredParam(t *testing.T) {
	s := newToolTestServer(t)
	// gmail_search_messages requires "query" param
	text, isError := callTool(t, s, "gmail_search_messages", nil)
	if !isError {
		t.Fatal("expected isError=true for missing query param")
	}
	if !strings.Contains(strings.ToLower(text), "query") {
		t.Errorf("expected error to mention 'query', got %q", text)
	}
	fmt.Println("OK: missing required param returns descriptive error:", text)
}
