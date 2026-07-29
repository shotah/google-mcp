package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	customsearch "google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"

	"github.com/shotah/google-mcp/server"
)

// RegisterSearchTools registers all Search tools with the MCP server.
func RegisterSearchTools(s *mcpserver.MCPServer, _ server.Config) {
	registerSearchCustom(s)
	registerGetSearchEngineInfo(s)
	registerSearchCustomSiterestrict(s)
}

// newCustomSearchService creates a customsearch.Service using the API key from env.
func newCustomSearchService(ctx context.Context) (*customsearch.Service, string, error) {
	apiKey := os.Getenv("GOOGLE_PSE_API_KEY")
	if apiKey == "" {
		return nil, "", errors.New("GOOGLE_PSE_API_KEY environment variable not set. Please set it to your Google Custom Search API key")
	}

	cx := os.Getenv("GOOGLE_PSE_ENGINE_ID")
	if cx == "" {
		return nil, "", errors.New("GOOGLE_PSE_ENGINE_ID environment variable not set. Please set it to your Programmable Search Engine ID")
	}

	svc, err := customsearch.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, "", fmt.Errorf("creating Custom Search service: %w", err)
	}
	return svc, cx, nil
}

// executeSearch runs a Custom Search query and formats the results.
func executeSearch(ctx context.Context, email, q string, num, start int, safe string, searchType, siteSearch, siteSearchFilter, dateRestrict, fileType, language, country string) (string, error) {
	svc, cx, err := newCustomSearchService(ctx)
	if err != nil {
		return "", err
	}

	call := svc.Cse.List().Q(q).Cx(cx).Num(int64(num)).Start(int64(start)).Safe(safe)

	if searchType != "" {
		call = call.SearchType(searchType)
	}
	if siteSearch != "" {
		call = call.SiteSearch(siteSearch)
	}
	if siteSearchFilter != "" {
		call = call.SiteSearchFilter(siteSearchFilter)
	}
	if dateRestrict != "" {
		call = call.DateRestrict(dateRestrict)
	}
	if fileType != "" {
		call = call.FileType(fileType)
	}
	if language != "" {
		call = call.Lr(language)
	}
	if country != "" {
		call = call.Cr(country)
	}

	result, err := call.Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("executing search: %w", err)
	}

	totalResults := "0"
	searchTime := 0.0
	if result.SearchInformation != nil {
		totalResults = result.SearchInformation.TotalResults
		searchTime = result.SearchInformation.SearchTime
	}

	itemCount := len(result.Items)

	var sb strings.Builder
	fmt.Fprintf(&sb, "Search Results for %s:\n", email)
	fmt.Fprintf(&sb, "- Query: \"%s\"\n", q)
	fmt.Fprintf(&sb, "- Search Engine ID: %s\n", cx)
	fmt.Fprintf(&sb, "- Total Results: %s\n", totalResults)
	fmt.Fprintf(&sb, "- Search Time: %.3f seconds\n", searchTime)
	fmt.Fprintf(&sb, "- Results Returned: %d (showing %d to %d)\n\n", itemCount, start, start+itemCount-1)

	if itemCount > 0 {
		sb.WriteString(formatCustomSearchResults(result.Items, start))
	} else {
		sb.WriteString("\nNo results found.")
	}

	// Pagination info
	if result.Queries != nil {
		if len(result.Queries.NextPage) > 0 {
			nextStart := result.Queries.NextPage[0].StartIndex
			fmt.Fprintf(&sb, "\n\nTo see more results, search again with start=%d", nextStart)
		}
	}

	return sb.String(), nil
}

// --- search_query ---

func registerSearchCustom(s *mcpserver.MCPServer) {
	tool := newMCPTool("search_query",
		mcp.WithDescription("Web search (titles, snippets, links). Use for general lookup. Not Drive/Gmail — use workspace tools. Site-limited: search_query_siterestrict."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("q", mcp.Required(), mcp.Description("Search query.")),
	)
	RegisterTool(s, tool, handleSearchCustom)
}

func handleSearchCustom(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := request.RequireString("q")
	if err != nil {
		return needArg("q", `search_query(q="…")`), nil
	}
	email, err := resolveEmail(request)
	if err != nil {
		return needArg("user_google_email", `search_query(q="…")`), nil
	}

	// Optional filters still accepted if a client sends them (not in lean schema).
	num := request.GetInt("num", 10)
	start := request.GetInt("start", 1)
	safe := request.GetString("safe", "off")
	searchType := request.GetString("search_type", "")
	siteSearch := request.GetString("site_search", "")
	siteSearchFilter := request.GetString("site_search_filter", "")
	dateRestrict := request.GetString("date_restrict", "")
	fileType := request.GetString("file_type", "")
	language := request.GetString("language", "")
	country := request.GetString("country", "")

	text, err := executeSearch(ctx, email, q, num, start, safe, searchType, siteSearch, siteSearchFilter, dateRestrict, fileType, language, country)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}

// --- search_get_engine_info ---

func registerGetSearchEngineInfo(s *mcpserver.MCPServer) {
	tool := newMCPTool("search_get_engine_info",
		mcp.WithDescription("Metadata about the configured Programmable Search Engine (cx). Use to verify search setup. Not for running a query — use search_query."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
	)
	RegisterTool(s, tool, handleGetSearchEngineInfo)
}

func handleGetSearchEngineInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	email, err := resolveEmail(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	svc, cx, err := newCustomSearchService(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Perform a minimal search to get the search engine context.
	result, err := svc.Cse.List().Q("test").Cx(cx).Num(1).Context(ctx).Do()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("querying search engine info: %v", err)), nil
	}

	// Context is googleapi.RawMessage (JSON bytes) — parse manually.
	title := "Unknown"
	var contextData map[string]any
	if len(result.Context) > 0 {
		_ = json.Unmarshal(result.Context, &contextData)
	}
	if contextData != nil {
		if t, ok := contextData["title"].(string); ok && t != "" {
			title = t
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Search Engine Information for %s:\n", email)
	fmt.Fprintf(&sb, "- Search Engine ID: %s\n", cx)
	fmt.Fprintf(&sb, "- Title: %s\n", title)

	// Add facet/refinement information if available.
	if contextData != nil {
		sb.WriteString(formatSearchFacets(contextData["facets"]))
	}

	// Add search statistics.
	if result.SearchInformation != nil {
		totalResults := result.SearchInformation.TotalResults
		if totalResults == "" {
			totalResults = "Unknown"
		}
		sb.WriteString("\nSearch Statistics:\n")
		fmt.Fprintf(&sb, "  - Total indexed results: %s\n", totalResults)
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func formatCustomSearchResults(items []*customsearch.Result, start int) string {
	var sb strings.Builder
	sb.WriteString("Results:\n")
	for i, item := range items {
		title := valueOrDefault(item.Title, "No title")
		link := valueOrDefault(item.Link, "No link")
		snippet := valueOrDefault(item.Snippet, "No description available")
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		fmt.Fprintf(&sb, "\n%d. %s\n   URL: %s\n   Snippet: %s\n", start+i, title, link, snippet)
		sb.WriteString(formatSearchResultMetadata(item.Pagemap))
	}
	return sb.String()
}

func formatSearchResultMetadata(raw []byte) string {
	var pagemap map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &pagemap) != nil {
		return ""
	}
	metatags, ok := pagemap["metatags"].([]any)
	if !ok || len(metatags) == 0 {
		return ""
	}
	meta, ok := metatags[0].(map[string]any)
	if !ok {
		return ""
	}
	var sb strings.Builder
	if ogType, ok := meta["og:type"].(string); ok {
		fmt.Fprintf(&sb, "   Type: %s\n", ogType)
	}
	if pubTime, ok := meta["article:published_time"].(string); ok && len(pubTime) >= 10 {
		fmt.Fprintf(&sb, "   Published: %s\n", pubTime[:10])
	}
	return sb.String()
}

func formatSearchFacets(raw any) string {
	facets, ok := raw.([]any)
	if !ok || len(facets) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nAvailable Refinements:\n")
	for _, facet := range facets {
		facetList, ok := facet.([]any)
		if !ok {
			continue
		}
		for _, item := range facetList {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			label, _ := itemMap["label"].(string)
			anchor, _ := itemMap["anchor"].(string)
			fmt.Fprintf(&sb, "  - %s (anchor: %s)\n", valueOrDefault(label, "Unknown"), valueOrDefault(anchor, "Unknown"))
		}
	}
	return sb.String()
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// --- search_query_siterestrict ---

func registerSearchCustomSiterestrict(s *mcpserver.MCPServer) {
	tool := newMCPTool("search_query_siterestrict",
		mcp.WithDescription("Web search limited to specific site(s) via Programmable Search. Use for 'search company.com for…'. Broader web: search_query."),
		mcp.WithString("user_google_email",
			mcp.Required(),
			mcp.Description("The user's Google email address."),
		),
		mcp.WithString("q",
			mcp.Required(),
			mcp.Description("The search query."),
		),
		mcp.WithArray("sites",
			mcp.Required(),
			mcp.Description("List of sites/domains to search within."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithNumber("num",
			mcp.Description("Number of results to return (1-10). Defaults to 10."),
		),
		mcp.WithNumber("start",
			mcp.Description("The index of the first result to return (1-based). Defaults to 1."),
		),
		mcp.WithString("safe",
			mcp.Description("Safe search level. Defaults to \"off\"."),
			mcp.Enum("active", "moderate", "off"),
		),
	)
	RegisterTool(s, tool, handleSearchCustomSiterestrict)
}

func handleSearchCustomSiterestrict(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	email, err := resolveEmail(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	q, err := request.RequireString("q")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	sites, err := request.RequireStringSlice("sites")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	num := request.GetInt("num", 10)
	start := request.GetInt("start", 1)
	safe := request.GetString("safe", "off")

	// Build site restriction query.
	siteParts := make([]string, len(sites))
	for i, site := range sites {
		siteParts[i] = "site:" + site
	}
	siteQuery := strings.Join(siteParts, " OR ")
	fullQuery := fmt.Sprintf("%s (%s)", q, siteQuery)

	text, err := executeSearch(ctx, email, fullQuery, num, start, safe, "", "", "", "", "", "", "")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}
