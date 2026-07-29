package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	mcpserver "github.com/mark3labs/mcp-go/server"

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

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseFlags(os.Args[1:])
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

// parseFlags parses CLI arguments into a server.Config.
func parseFlags(args []string) (server.Config, error) {
	fs := flag.NewFlagSet("google-mcp", flag.ContinueOnError)

	var toolsRaw string
	fs.StringVar(&toolsRaw, "tools", "", "space-separated list of services to enable (e.g. gmail drive calendar)")
	var toolTier string
	fs.StringVar(&toolTier, "tool-tier", "", "tool depth: core, extended, or complete (default: complete)")
	var capability string
	fs.StringVar(&capability, "capability", "", "permission surface: read, edit, or complete (default: complete)")
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
