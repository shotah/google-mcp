package tools

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// extractGoogleResourceID extracts a Google resource ID from a Docs/Sheets/Slides/
// Drive/Forms share or edit URL, or returns the trimmed input if it is already an ID.
// Returns "" when the input looks like a URL but no Google ID can be parsed.
func extractGoogleResourceID(input string) string {
	input = strings.TrimSpace(input)
	input = strings.Trim(input, "<>\"'")
	if input == "" {
		return ""
	}
	if looksLikeGoogleURL(input) {
		return parseGoogleURLID(input)
	}
	if looksLikeAnyURL(input) {
		return ""
	}
	return input
}

// parseGoogleURLID extracts an id from a Google share/edit URL path or query.
func parseGoogleURLID(input string) string {
	for _, pattern := range []string{"/file/d/", "/folders/", "/d/"} {
		if id := cutGooglePathID(input, pattern); id != "" {
			return id
		}
	}
	return idFromGoogleQuery(input)
}

func idFromGoogleQuery(input string) string {
	if u, err := url.Parse(ensureURLScheme(input)); err == nil {
		if id := u.Query().Get("id"); id != "" {
			return id
		}
	}
	_, after, ok := strings.Cut(input, "id=")
	if !ok {
		return ""
	}
	before, _, _ := strings.Cut(after, "&")
	before, _, _ = strings.Cut(before, "#")
	return before
}

func looksLikeGoogleURL(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(strings.Trim(input, "<>\"'")))
	return strings.Contains(lower, "docs.google.com/") ||
		strings.Contains(lower, "drive.google.com/") ||
		strings.Contains(lower, "forms.gle/")
}

func looksLikeAnyURL(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")
}

func ensureURLScheme(input string) string {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return input
	}
	return "https://" + input
}

func cutGooglePathID(input, pattern string) string {
	_, after, ok := strings.Cut(input, pattern)
	if !ok {
		// Case-insensitive fallback for unusual pastes
		idx := strings.Index(strings.ToLower(input), pattern)
		if idx < 0 {
			return ""
		}
		after = input[idx+len(pattern):]
	}
	if after == "" {
		return ""
	}
	if end := strings.IndexAny(after, "/?#&"); end >= 0 {
		after = after[:end]
	}
	return after
}

// requireGoogleID reads a required string arg and normalizes share/edit URLs to raw IDs.
func requireGoogleID(request mcp.CallToolRequest, key string) (string, error) {
	raw, err := request.RequireString(key)
	if err != nil {
		return "", err
	}
	id := extractGoogleResourceID(raw)
	if id == "" {
		return "", fmt.Errorf("%s: could not parse a Google id from %q — paste a share/edit URL or raw id", key, raw)
	}
	return id, nil
}

// googleIDError maps requireGoogleID failures to a tool result (parse error vs missing arg).
func googleIDError(err error, key, nextCall string) *mcp.CallToolResult {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "could not parse") {
		return mcp.NewToolResultError(err.Error())
	}
	return needArg(key, nextCall)
}

// optionalFolderID reads optional folder_id (with default) and normalizes share URLs.
func optionalFolderID(request mcp.CallToolRequest, defaultVal string) string {
	raw := strings.TrimSpace(request.GetString("folder_id", defaultVal))
	if raw == "" {
		return defaultVal
	}
	if id := extractGoogleResourceID(raw); id != "" {
		return id
	}
	return raw
}
