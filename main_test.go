package main

import (
	"testing"

	"github.com/shotah/google-mcp/server"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("transport = %q, want %q", cfg.Transport, "stdio")
	}
	if len(cfg.Tools) != 0 {
		t.Errorf("tools = %v, want empty", cfg.Tools)
	}
	if cfg.ToolTier != "" {
		t.Errorf("tool-tier = %q, want empty", cfg.ToolTier)
	}
	if cfg.SingleUser {
		t.Error("single-user should default to false")
	}
	if cfg.ReadOnly {
		t.Error("read-only should default to false")
	}
}

func TestParseFlagsAllFlags(t *testing.T) {
	args := []string{
		"--tools", "gmail drive",
		"--tool-tier", "core",
		"--capability", "edit",
		"--transport", "stdio",
		"--single-user",
		"--read-only",
	}
	cfg, err := parseFlags(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := server.Config{
		Tools:      []string{"gmail", "drive"},
		ToolTier:   "core",
		Capability: "edit",
		Transport:  "stdio",
		SingleUser: true,
		ReadOnly:   true,
	}
	if len(cfg.Tools) != len(want.Tools) {
		t.Fatalf("tools len = %d, want %d", len(cfg.Tools), len(want.Tools))
	}
	for i, tool := range cfg.Tools {
		if tool != want.Tools[i] {
			t.Errorf("tools[%d] = %q, want %q", i, tool, want.Tools[i])
		}
	}
	if cfg.ToolTier != want.ToolTier {
		t.Errorf("tool-tier = %q, want %q", cfg.ToolTier, want.ToolTier)
	}
	if cfg.Capability != want.Capability {
		t.Errorf("capability = %q, want %q", cfg.Capability, want.Capability)
	}
	if cfg.Transport != want.Transport {
		t.Errorf("transport = %q, want %q", cfg.Transport, want.Transport)
	}
	if cfg.SingleUser != want.SingleUser {
		t.Errorf("single-user = %v, want %v", cfg.SingleUser, want.SingleUser)
	}
	if cfg.ReadOnly != want.ReadOnly {
		t.Errorf("read-only = %v, want %v", cfg.ReadOnly, want.ReadOnly)
	}
}

func TestParseFlagsStreamableHTTP(t *testing.T) {
	cfg, err := parseFlags([]string{"--transport", "streamable-http"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Transport != "streamable-http" {
		t.Errorf("transport = %q, want %q", cfg.Transport, "streamable-http")
	}
}

func TestParseFlagsInvalidTool(t *testing.T) {
	_, err := parseFlags([]string{"--tools", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid tool")
	}
}

func TestParseFlagsInvalidTier(t *testing.T) {
	_, err := parseFlags([]string{"--tool-tier", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid tier")
	}
}

func TestParseFlagsInvalidCapability(t *testing.T) {
	_, err := parseFlags([]string{"--capability", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid capability")
	}
}

func TestParseFlagsCapabilityValues(t *testing.T) {
	for _, cap := range []string{"read", "edit", "complete"} {
		cfg, err := parseFlags([]string{"--capability", cap})
		if err != nil {
			t.Errorf("unexpected error for capability %q: %v", cap, err)
		}
		if cfg.Capability != cap {
			t.Errorf("capability = %q, want %q", cfg.Capability, cap)
		}
	}
}

func TestParseFlagsInvalidTransport(t *testing.T) {
	_, err := parseFlags([]string{"--transport", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid transport")
	}
}

func TestParseFlagsToolTierValues(t *testing.T) {
	for _, tier := range []string{"core", "extended", "complete"} {
		cfg, err := parseFlags([]string{"--tool-tier", tier})
		if err != nil {
			t.Errorf("unexpected error for tier %q: %v", tier, err)
		}
		if cfg.ToolTier != tier {
			t.Errorf("tool-tier = %q, want %q", cfg.ToolTier, tier)
		}
	}
}

func TestParseFlagsAllToolNames(t *testing.T) {
	all := "gmail drive calendar docs sheets chat forms slides tasks contacts search appscript"
	cfg, err := parseFlags([]string{"--tools", all})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Tools) != 12 {
		t.Errorf("tools len = %d, want 12", len(cfg.Tools))
	}
}

func TestParseFlagsNoFlagsAllToolsLoaded(t *testing.T) {
	cfg, err := parseFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No --tools flag means empty slice (all tools loaded by convention).
	if len(cfg.Tools) != 0 {
		t.Errorf("tools = %v, want empty (all tools)", cfg.Tools)
	}
}

func TestParseFlagsPresetLean(t *testing.T) {
	cfg, err := parseFlags([]string{"--preset", "lean"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTools := []string{"gmail", "calendar"}
	if len(cfg.Tools) != len(wantTools) {
		t.Fatalf("tools = %v, want %v", cfg.Tools, wantTools)
	}
	for i, tool := range wantTools {
		if cfg.Tools[i] != tool {
			t.Errorf("tools[%d] = %q, want %q", i, cfg.Tools[i], tool)
		}
	}
	if cfg.ToolTier != "core" {
		t.Errorf("tool-tier = %q, want %q", cfg.ToolTier, "core")
	}
	if cfg.Capability != "edit" {
		t.Errorf("capability = %q, want %q", cfg.Capability, "edit")
	}
}

func TestParseFlagsPresetLeanOverrides(t *testing.T) {
	cfg, err := parseFlags([]string{
		"--preset", "lean",
		"--tools", "gmail calendar tasks",
		"--tool-tier", "extended",
		"--capability", "read",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTools := []string{"gmail", "calendar", "tasks"}
	if len(cfg.Tools) != len(wantTools) {
		t.Fatalf("tools = %v, want %v", cfg.Tools, wantTools)
	}
	for i, tool := range wantTools {
		if cfg.Tools[i] != tool {
			t.Errorf("tools[%d] = %q, want %q", i, cfg.Tools[i], tool)
		}
	}
	if cfg.ToolTier != "extended" {
		t.Errorf("tool-tier = %q, want %q", cfg.ToolTier, "extended")
	}
	if cfg.Capability != "read" {
		t.Errorf("capability = %q, want %q", cfg.Capability, "read")
	}
}

func TestParseFlagsPresetEveryday(t *testing.T) {
	cfg, err := parseFlags([]string{"--preset", "everyday"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTools := []string{"gmail", "calendar", "docs", "sheets", "tasks", "contacts", "drive"}
	if len(cfg.Tools) != len(wantTools) {
		t.Fatalf("tools = %v, want %v", cfg.Tools, wantTools)
	}
	for i, tool := range wantTools {
		if cfg.Tools[i] != tool {
			t.Errorf("tools[%d] = %q, want %q", i, cfg.Tools[i], tool)
		}
	}
	if cfg.ToolTier != "core" || cfg.Capability != "edit" {
		t.Errorf("tier/capability = %q/%q, want core/edit", cfg.ToolTier, cfg.Capability)
	}
}

func TestParseFlagsInvalidPreset(t *testing.T) {
	_, err := parseFlags([]string{"--preset", "bogus"})
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}
}

func TestRunAuthCommand_DispatchURL(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "test-cid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "test-csecret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URI", "")
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())

	err := runAuthCommand([]string{"url"})
	if err != nil {
		t.Fatalf("runAuthCommand url: %v", err)
	}
}

func TestRunAuthCommand_ExchangeMissingCode(t *testing.T) {
	err := runAuthCommand([]string{"exchange"})
	if err == nil {
		t.Fatal("expected error for exchange without code")
	}
}

func TestRunAuthCommand_ExchangeNoPending(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "cid")
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "csecret")
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())

	err := runAuthCommand([]string{"exchange", "some-code"})
	if err == nil {
		t.Fatal("expected error when no pending session exists")
	}
}

func TestParseAuthArgs(t *testing.T) {
	t.Setenv("USER_GOOGLE_EMAIL", "")

	got, err := parseAuthArgs([]string{"--email", "a@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "a@example.com" {
		t.Errorf("got %q, want a@example.com", got)
	}

	got, err = parseAuthArgs([]string{"b@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "b@example.com" {
		t.Errorf("got %q, want b@example.com", got)
	}

	t.Setenv("USER_GOOGLE_EMAIL", "env@example.com")
	got, err = parseAuthArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "env@example.com" {
		t.Errorf("got %q, want env@example.com", got)
	}
}
