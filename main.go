package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/shotah/google-mcp/auth"
	"github.com/shotah/google-mcp/server"
	"github.com/shotah/google-mcp/tools"
)

// validTools is the set of accepted --tools values.
var validTools = map[string]bool{
	"gmail": true, "drive": true, "calendar": true,
	"docs": true, "sheets": true, "chat": true,
	"forms": true, "slides": true, "tasks": true,
	"contacts": true, "search": true, "appscript": true,
}

// validTiers is the set of accepted --tool-tier values.
var validTiers = map[string]bool{
	"core": true, "extended": true, "complete": true,
}

// validCapabilities is the set of accepted --capability values.
var validCapabilities = map[string]bool{
	"read": true, "edit": true, "complete": true,
}

// validTransports is the set of accepted --transport values.
var validTransports = map[string]bool{
	"stdio": true, "streamable-http": true,
}

// validPresets is the set of accepted --preset values.
var validPresets = map[string]bool{
	"lean":     true,
	"everyday": true,
}

// presetDefaults maps a preset name to tools / tier / capability.
// Explicit --tools / --tool-tier / --capability override the matching fields.
var presetDefaults = map[string]struct {
	tools      []string
	toolTier   string
	capability string
}{
	// Smallest surface for tiny local models (~11 tools): mail + calendar only.
	"lean": {
		tools:      []string{"gmail", "calendar"},
		toolTier:   "core",
		capability: "edit",
	},
	// Personal assistant (~21 tools): mail + calendar + docs + sheets + tasks.
	// Drive is usually unnecessary for Docs/Sheets work — see README.
	"everyday": {
		tools:      []string{"gmail", "calendar", "docs", "sheets", "tasks"},
		toolTier:   "core",
		capability: "edit",
	},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "auth", "login":
			return runAuthCommand(args[1:])
		}
	}

	cfg, err := parseFlags(args)
	if err != nil {
		return err
	}

	s := server.New(cfg)
	tools.RegisterAllTools(s, cfg)
	tools.FilterTools(s, cfg)

	switch cfg.Transport {
	case "streamable-http":
		fmt.Fprintln(os.Stderr, "streamable-http transport is not yet implemented")
		return nil
	default:
		errLogger := log.New(os.Stderr, "", log.LstdFlags)
		return mcpserver.ServeStdio(s, mcpserver.WithErrorLogger(errLogger))
	}
}

// runAuthCommand performs first-time (or re-auth) Google OAuth for humans.
// Usage: google-mcp auth [--email you@gmail.com]
//
//	google-mcp auth you@gmail.com
//	google-mcp login  (alias)
func runAuthCommand(args []string) error {
	email, err := parseAuthArgs(args)
	if err != nil {
		return err
	}

	store := auth.NewCredentialStore()
	gotEmail, path, err := auth.RunInteractiveAuth(context.Background(), email, store, auth.OpenBrowser)
	if err != nil {
		return err
	}

	fmt.Printf("Authenticated as %s\n", gotEmail)
	fmt.Printf("Credentials saved to %s\n", path)
	fmt.Println("Copy this file onto the agent host if needed, then start the MCP server (e.g. google-mcp --preset lean).")
	fmt.Println("Access tokens refresh automatically; you do not need to call auth_start from the agent.")
	return nil
}

// parseAuthArgs resolves the optional email for `google-mcp auth`.
func parseAuthArgs(args []string) (string, error) {
	fs := flag.NewFlagSet("google-mcp auth", flag.ContinueOnError)
	var email string
	fs.StringVar(&email, "email", "", "Google account email (optional; defaults to USER_GOOGLE_EMAIL or the account chosen in the browser)")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if email == "" && fs.NArg() == 1 {
		email = fs.Arg(0)
	}
	if email == "" {
		email = strings.TrimSpace(os.Getenv("USER_GOOGLE_EMAIL"))
	}
	return email, nil
}

// parseFlags parses CLI arguments into a server.Config.
func parseFlags(args []string) (server.Config, error) {
	fs := flag.NewFlagSet("google-mcp", flag.ContinueOnError)

	var toolsRaw string
	fs.StringVar(&toolsRaw, "tools", "", "space-separated list of services to enable (e.g. gmail drive calendar)")
	var toolTier string
	fs.StringVar(&toolTier, "tool-tier", "", "tool depth: core, extended, or complete (default: complete)")
	var capability string
	fs.StringVar(&capability, "capability", "", "permission surface: read, edit, or complete (default: complete)")
	var preset string
	fs.StringVar(&preset, "preset", "", "named surface: lean (~11: gmail+calendar) or everyday (~21: +docs+sheets+tasks). Explicit --tools/--tool-tier/--capability override.")
	var transport string
	fs.StringVar(&transport, "transport", "stdio", "transport mode: stdio or streamable-http")
	var singleUser bool
	fs.BoolVar(&singleUser, "single-user", false, "enable single-user mode")
	var readOnly bool
	fs.BoolVar(&readOnly, "read-only", false, "shorthand for --capability read (no write/delete tools)")

	if err := fs.Parse(args); err != nil {
		return server.Config{}, err
	}

	// Validate and collect tools.
	var selectedTools []string
	if toolsRaw != "" {
		for t := range strings.FieldsSeq(toolsRaw) {
			if !validTools[t] {
				return server.Config{}, fmt.Errorf("unknown tool %q; valid tools: gmail, drive, calendar, docs, sheets, chat, forms, slides, tasks, contacts, search, appscript", t)
			}
			selectedTools = append(selectedTools, t)
		}
	}

	// Validate tool tier.
	if toolTier != "" && !validTiers[toolTier] {
		return server.Config{}, fmt.Errorf("unknown tool-tier %q; valid tiers: core, extended, complete", toolTier)
	}

	// Validate capability.
	if capability != "" && !validCapabilities[capability] {
		return server.Config{}, fmt.Errorf("unknown capability %q; valid: read, edit, complete", capability)
	}

	// Validate and apply preset (fills only unset tools/tier/capability).
	if preset != "" {
		if !validPresets[preset] {
			return server.Config{}, fmt.Errorf("unknown preset %q; valid presets: lean, everyday", preset)
		}
		def := presetDefaults[preset]
		if len(selectedTools) == 0 {
			selectedTools = append([]string(nil), def.tools...)
		}
		if toolTier == "" {
			toolTier = def.toolTier
		}
		if capability == "" {
			capability = def.capability
		}
	}

	// Validate transport.
	if !validTransports[transport] {
		return server.Config{}, fmt.Errorf("unknown transport %q; valid transports: stdio, streamable-http", transport)
	}

	return server.Config{
		Tools:      selectedTools,
		ToolTier:   toolTier,
		Capability: capability,
		Transport:  transport,
		SingleUser: singleUser,
		ReadOnly:   readOnly,
	}, nil
}
