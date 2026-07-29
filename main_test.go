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
