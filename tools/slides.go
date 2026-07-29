package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	slides "google.golang.org/api/slides/v1"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// mapToStruct converts a map[string]any to a typed struct via JSON round-trip.
func mapToStruct(m map[string]any, out any) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// RegisterSlidesTools registers all Slides tools with the MCP server.
func RegisterSlidesTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	registerCreatePresentation(s, getClient)
	registerGetPresentation(s, getClient)
	registerBatchUpdatePresentation(s, getClient)
	registerGetPage(s, getClient)
	registerGetPageThumbnail(s, getClient)

	// Register comment tools for Slides (US-006 / US-019).
	RegisterCommentTools(s, getClient, "slides", "presentation", "presentation_id")
}

// newSlidesService creates a slides.Service for the given user email.
func newSlidesService(ctx context.Context, getClient httpClientFunc, email string) (*slides.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := slides.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Slides service: %w", err)
	}
	return svc, nil
}

// --- slides_create_presentation ---

func registerCreatePresentation(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("slides_create_presentation",
		mcp.WithDescription("Create a new Google Slides deck (title). Returns presentation_id. Use for 'new slideshow'. Add slides/content with slides_batch_update."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("title", mcp.Description("The title for the new presentation. Defaults to \"Untitled Presentation\".")),
	)
	s.AddTool(tool, handleCreatePresentation(getClient))
}

func handleCreatePresentation(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		title := request.GetString("title", "Untitled Presentation")

		svc, err := newSlidesService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pres := &slides.Presentation{Title: title}
		created, err := svc.Presentations.Create(pres).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating presentation: %v", err)), nil
		}

		presID := created.PresentationId
		presURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presID)
		slideCount := len(created.Slides)

		result := fmt.Sprintf(`Presentation Created Successfully for %s:
- Title: %s
- Presentation ID: %s
- URL: %s
- Slides: %d slide(s) created`, email, title, presID, presURL, slideCount)

		return mcp.NewToolResultText(result), nil
	}
}

// --- slides_get_presentation ---

func registerGetPresentation(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("slides_get_presentation",
		mcp.WithDescription("READ presentation metadata and slide/page ids for presentation_id. Use before slides_get_page or slides_batch_update. Not slide image — use slides_get_page_thumbnail."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("presentation_id", mcp.Required(), mcp.Description("The ID of the presentation to retrieve.")),
	)
	s.AddTool(tool, handleGetPresentation(getClient))
}

func handleGetPresentation(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		presID, err := request.RequireString("presentation_id")
		if err != nil {
			return mcp.NewToolResultError("presentation_id is required"), nil
		}

		svc, err := newSlidesService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pres, err := svc.Presentations.Get(presID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting presentation: %v", err)), nil
		}

		title := pres.Title
		if title == "" {
			title = "Untitled"
		}

		// Page size info
		pageSizeStr := "Unknown"
		if pres.PageSize != nil && pres.PageSize.Width != nil && pres.PageSize.Height != nil {
			pageSizeStr = fmt.Sprintf("%v x %v %s",
				pres.PageSize.Width.Magnitude,
				pres.PageSize.Height.Magnitude,
				pres.PageSize.Width.Unit)
		}

		// Build slides breakdown
		var slidesInfo []string
		for i, slide := range pres.Slides {
			slideID := slide.ObjectId
			if slideID == "" {
				slideID = "Unknown"
			}
			elemCount := len(slide.PageElements)

			slideText := extractSlideText(slide)

			if slideText != "" {
				slidesInfo = append(slidesInfo, fmt.Sprintf("  Slide %d: ID %s, %d element(s), text: %s",
					i+1, slideID, elemCount, slideText))
			} else {
				slidesInfo = append(slidesInfo, fmt.Sprintf("  Slide %d: ID %s, %d element(s), text: empty",
					i+1, slideID, elemCount))
			}
		}

		slidesBreakdown := "  No slides found"
		if len(slidesInfo) > 0 {
			slidesBreakdown = strings.Join(slidesInfo, "\n")
		}

		presURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presID)

		result := fmt.Sprintf(`Presentation Details for %s:
- Title: %s
- Presentation ID: %s
- URL: %s
- Total Slides: %d
- Page Size: %s

Slides Breakdown:
%s`, email, title, presID, presURL, len(pres.Slides), pageSizeStr, slidesBreakdown)

		return mcp.NewToolResultText(result), nil
	}
}

// extractSlideText extracts text content from a slide's page elements.
// It walks shape text elements, sorts by startIndex, and joins text runs.
func extractSlideText(slide *slides.Page) string {
	var textsFromElements []string

	for _, elem := range slide.PageElements {
		if elem.Shape == nil || elem.Shape.Text == nil {
			continue
		}

		type indexedText struct {
			index   int64
			content string
		}
		var textRuns []indexedText

		for _, te := range elem.Shape.Text.TextElements {
			if te.TextRun == nil || te.TextRun.Content == "" {
				continue
			}
			textRuns = append(textRuns, indexedText{
				index:   te.StartIndex,
				content: te.TextRun.Content,
			})
		}

		if len(textRuns) > 0 {
			sort.Slice(textRuns, func(i, j int) bool {
				return textRuns[i].index < textRuns[j].index
			})
			var sb strings.Builder
			for _, tr := range textRuns {
				sb.WriteString(tr.content)
			}
			textsFromElements = append(textsFromElements, sb.String())
		}
	}

	if len(textsFromElements) == 0 {
		return ""
	}

	// Clean up: split on newlines, filter empty, prefix with >
	combined := strings.Join(textsFromElements, "\n")
	lines := strings.Split(combined, "\n")
	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, "    > "+line)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	return "\n" + strings.Join(nonEmpty, "\n")
}

// --- slides_batch_update ---

func registerBatchUpdatePresentation(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("slides_batch_update",
		mcp.WithDescription("Apply Slides API batch updates on presentation_id (create slides, insert text/shapes, etc.). Use for 'edit my deck'. Prefer slides_get_presentation for ids first."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("presentation_id", mcp.Required(), mcp.Description("The ID of the presentation to update.")),
		mcp.WithArray("requests", mcp.Required(), mcp.Description("List of update requests to apply."), mcp.Items(map[string]any{"type": "object"})),
	)
	s.AddTool(tool, handleBatchUpdatePresentation(getClient))
}

func handleBatchUpdatePresentation(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		presID, err := request.RequireString("presentation_id")
		if err != nil {
			return mcp.NewToolResultError("presentation_id is required"), nil
		}

		args := request.GetArguments()
		rawRequests, ok := args["requests"]
		if !ok {
			return mcp.NewToolResultError("requests is required"), nil
		}
		requestsList, ok := rawRequests.([]any)
		if !ok {
			return mcp.NewToolResultError("requests must be an array"), nil
		}

		svc, err := newSlidesService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Convert the raw request maps into slides.Request objects.
		// Since the Slides API request types are complex and varied, we use
		// a JSON round-trip approach: marshal the raw maps to JSON, then
		// unmarshal into the typed slides.Request structs.
		var reqs []*slides.Request
		for _, raw := range requestsList {
			reqMap, ok := raw.(map[string]any)
			if !ok {
				return mcp.NewToolResultError("each request must be an object"), nil
			}
			req := &slides.Request{}
			if err := mapToStruct(reqMap, req); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("parsing request: %v", err)), nil
			}
			reqs = append(reqs, req)
		}

		batchReq := &slides.BatchUpdatePresentationRequest{
			Requests: reqs,
		}

		resp, err := svc.Presentations.BatchUpdate(presID, batchReq).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch updating presentation: %v", err)), nil
		}

		presURL := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", presID)

		var out strings.Builder
		fmt.Fprintf(&out, "Batch Update Completed for %s:\n", email)
		fmt.Fprintf(&out, "- Presentation ID: %s\n", presID)
		fmt.Fprintf(&out, "- URL: %s\n", presURL)
		fmt.Fprintf(&out, "- Requests Applied: %d\n", len(requestsList))
		fmt.Fprintf(&out, "- Replies Received: %d", len(resp.Replies))

		if len(resp.Replies) > 0 {
			out.WriteString(formatSlidesUpdateReplies(resp.Replies))
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

// --- slides_get_page ---

func registerGetPage(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("slides_get_page",
		mcp.WithDescription("Get one slide/page details (elements, layout) by presentation_id + page_id. page_id from slides_get_presentation. Not thumbnail — use slides_get_page_thumbnail."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("presentation_id", mcp.Required(), mcp.Description("The ID of the presentation.")),
		mcp.WithString("page_object_id", mcp.Required(), mcp.Description("The object ID of the page/slide to retrieve.")),
	)
	s.AddTool(tool, handleGetPage(getClient))
}

func handleGetPage(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		presID, err := request.RequireString("presentation_id")
		if err != nil {
			return mcp.NewToolResultError("presentation_id is required"), nil
		}
		pageID, err := request.RequireString("page_object_id")
		if err != nil {
			return mcp.NewToolResultError("page_object_id is required"), nil
		}

		svc, err := newSlidesService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		page, err := svc.Presentations.Pages.Get(presID, pageID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting page: %v", err)), nil
		}

		pageType := page.PageType
		if pageType == "" {
			pageType = "Unknown"
		}

		var elementsInfo []string
		for _, elem := range page.PageElements {
			elementsInfo = append(elementsInfo, formatPageElement(elem))
		}

		elementsBreakdown := "  No elements found"
		if len(elementsInfo) > 0 {
			elementsBreakdown = strings.Join(elementsInfo, "\n")
		}

		result := fmt.Sprintf(`Page Details for %s:
- Presentation ID: %s
- Page ID: %s
- Page Type: %s
- Total Elements: %d

Page Elements:
%s`, email, presID, pageID, pageType, len(page.PageElements), elementsBreakdown)

		return mcp.NewToolResultText(result), nil
	}
}

func formatSlidesUpdateReplies(replies []*slides.Response) string {
	var out strings.Builder
	out.WriteString("\n\nUpdate Results:")
	for i, reply := range replies {
		switch {
		case reply.CreateSlide != nil:
			fmt.Fprintf(&out, "\n  Request %d: Created slide with ID %s", i+1, valueOrUnknown(reply.CreateSlide.ObjectId))
		case reply.CreateShape != nil:
			fmt.Fprintf(&out, "\n  Request %d: Created shape with ID %s", i+1, valueOrUnknown(reply.CreateShape.ObjectId))
		default:
			fmt.Fprintf(&out, "\n  Request %d: Operation completed", i+1)
		}
	}
	return out.String()
}

func formatPageElement(elem *slides.PageElement) string {
	elemID := valueOrUnknown(elem.ObjectId)
	switch {
	case elem.Shape != nil:
		return fmt.Sprintf("  Shape: ID %s, Type: %s", elemID, valueOrUnknown(elem.Shape.ShapeType))
	case elem.Table != nil:
		return fmt.Sprintf("  Table: ID %s, Size: %dx%d", elemID, elem.Table.Rows, elem.Table.Columns)
	case elem.Line != nil:
		return fmt.Sprintf("  Line: ID %s, Type: %s", elemID, valueOrUnknown(elem.Line.LineType))
	default:
		return fmt.Sprintf("  Element: ID %s, Type: Unknown", elemID)
	}
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "Unknown"
	}
	return value
}

// --- slides_get_page_thumbnail ---

func registerGetPageThumbnail(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("slides_get_page_thumbnail",
		mcp.WithDescription("Thumbnail image URL for presentation_id + page_id. Use when user needs a slide preview. Not full slide JSON — use slides_get_page."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("presentation_id", mcp.Required(), mcp.Description("The ID of the presentation.")),
		mcp.WithString("page_object_id", mcp.Required(), mcp.Description("The object ID of the page/slide.")),
		mcp.WithString("thumbnail_size", mcp.Description("Size of thumbnail (\"LARGE\", \"MEDIUM\", \"SMALL\"). Defaults to \"MEDIUM\"."), mcp.Enum("LARGE", "MEDIUM", "SMALL")),
	)
	s.AddTool(tool, handleGetPageThumbnail(getClient))
}

func handleGetPageThumbnail(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		presID, err := request.RequireString("presentation_id")
		if err != nil {
			return mcp.NewToolResultError("presentation_id is required"), nil
		}
		pageID, err := request.RequireString("page_object_id")
		if err != nil {
			return mcp.NewToolResultError("page_object_id is required"), nil
		}
		thumbnailSize := request.GetString("thumbnail_size", "MEDIUM")

		svc, err := newSlidesService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		thumbnail, err := svc.Presentations.Pages.GetThumbnail(presID, pageID).
			ThumbnailPropertiesThumbnailSize(thumbnailSize).
			ThumbnailPropertiesMimeType("PNG").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting page thumbnail: %v", err)), nil
		}

		thumbnailURL := thumbnail.ContentUrl

		result := fmt.Sprintf(`Thumbnail Generated for %s:
- Presentation ID: %s
- Page ID: %s
- Thumbnail Size: %s
- Thumbnail URL: %s

You can view or download the thumbnail using the provided URL.`, email, presID, pageID, thumbnailSize, thumbnailURL)

		return mcp.NewToolResultText(result), nil
	}
}
