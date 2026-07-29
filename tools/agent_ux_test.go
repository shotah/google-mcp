package tools

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewMCPToolReadOnlyAnnotation(t *testing.T) {
	tool := newMCPTool("calendar_list_events",
		mcp.WithDescription("list"),
	)
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Fatal("expected readOnlyHint=true for calendar_list_events")
	}
}

func TestNewMCPToolDestructiveAnnotation(t *testing.T) {
	tool := newMCPTool("calendar_delete_event",
		mcp.WithDescription("delete"),
	)
	if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
		t.Fatal("expected destructiveHint=true for calendar_delete_event")
	}

	resolve := newMCPTool("docs_resolve_comment",
		mcp.WithDescription("resolve"),
	)
	if resolve.Annotations.DestructiveHint == nil || !*resolve.Annotations.DestructiveHint {
		t.Fatal("expected destructiveHint=true for docs_resolve_comment")
	}
}

func TestNeedArgTeachesNextCall(t *testing.T) {
	res := needArg("event_id", "calendar_get_event(event_id=…)")
	if res == nil || !res.IsError {
		t.Fatal("expected error result")
	}
	text := ""
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(mcp.TextContent); ok {
			text = tc.Text
		}
	}
	if !strings.Contains(text, "event_id is required") {
		t.Fatalf("missing arg name: %q", text)
	}
	if !strings.Contains(text, "Next: calendar_get_event") {
		t.Fatalf("missing next call: %q", text)
	}
}
