package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	docs "google.golang.org/api/docs/v1"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// RegisterDocsTools registers all Docs tools with the MCP server.
func RegisterDocsTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools (US-013)
	registerSearchDocs(s, getClient)
	registerGetDocContent(s, getClient)
	registerListDocsInFolder(s, getClient)
	registerInspectDocStructure(s, getClient)
	registerDebugTableStructure(s, getClient)
	registerExportDocToPDF(s, getClient)

	// Write tools (US-013 — docs_create is categorized here)
	registerCreateDoc(s, getClient)

	// Write tools (US-014)
	registerModifyDocText(s, getClient)
	registerFindAndReplaceDoc(s, getClient)
	registerInsertDocElements(s, getClient)
	registerInsertDocImage(s, getClient)
	registerUpdateDocHeadersFooters(s, getClient)
	registerBatchUpdateDoc(s, getClient)
	registerCreateTableWithData(s, getClient)
	registerUpdateParagraphStyle(s, getClient)

	// Register comment tools for Docs (US-006 / US-019).
	RegisterCommentTools(s, getClient, "docs", "document", "document_id")
}

// newDocsService creates a docs.Service for the given user email.
func newDocsService(ctx context.Context, getClient httpClientFunc, email string) (*docs.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := docs.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Docs service: %w", err)
	}
	return svc, nil
}

// --- docs_search ---

func registerSearchDocs(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_search",
		mcp.WithDescription("Find Google Docs by title (returns document_id). Paste a share URL to resolve one doc. Use for 'find my doc called…'. Then docs_get_content. In-folder browse: docs_list_in_folder."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Document title text or Docs/Drive share URL.")),
		mcp.WithNumber("page_size", mcp.Description("Number of results to return. Defaults to 10.")),
	)
	s.AddTool(tool, handleSearchDocs(getClient))
}

func handleSearchDocs(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}
		pageSize := request.GetInt("page_size", 10)

		driveSvc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var files []*drive.File
		if looksLikeGoogleURL(query) {
			fileID := extractGoogleResourceID(query)
			if fileID == "" {
				return mcp.NewToolResultError("query looks like a URL but no Google document id was found"), nil
			}
			meta, err := driveSvc.Files.Get(fileID).
				Fields("id, name, createdTime, modifiedTime, webViewLink").
				SupportsAllDrives(true).
				Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
			}
			files = []*drive.File{meta}
		} else {
			escaped := strings.ReplaceAll(query, "'", "\\'")
			q := fmt.Sprintf("name contains '%s' and mimeType='application/vnd.google-apps.document' and trashed=false", escaped)

			resp, err := driveSvc.Files.List().
				Q(q).
				PageSize(int64(pageSize)).
				Fields("files(id, name, createdTime, modifiedTime, webViewLink)").
				SupportsAllDrives(true).
				IncludeItemsFromAllDrives(true).
				Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
			}
			files = resp.Files
		}

		if len(files) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No Google Docs found matching '%s'.", query)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d Google Docs matching '%s':", len(files), query)
		for _, f := range files {
			modified := f.ModifiedTime
			if modified == "" {
				modified = "N/A"
			}
			link := f.WebViewLink
			if link == "" {
				link = "#"
			}
			fmt.Fprintf(&b, "\n- %s (ID: %s) Modified: %s Link: %s", f.Name, f.Id, modified, link)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- docs_get_content ---

func registerGetDocContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_get_content",
		mcp.WithDescription("Read plain text by document_id or share URL. Edit → docs_modify_text. New doc → docs_create. Find by title → docs_search."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("Document id or Docs/Drive share URL.")),
	)
	s.AddTool(tool, handleGetDocContent(getClient))
}

func handleGetDocContent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "docs_get_content(document_id=…)"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "docs_create(title=…) or docs_get_content(document_id=…)"), nil
		}

		driveSvc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Get file metadata
		fileMeta, err := driveSvc.Files.Get(documentID).
			Fields("id, name, mimeType, webViewLink").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		mimeType := fileMeta.MimeType
		fileName := fileMeta.Name
		if fileName == "" {
			fileName = "Unknown File"
		}
		webViewLink := fileMeta.WebViewLink
		if webViewLink == "" {
			webViewLink = "#"
		}

		var bodyText string

		switch mimeType {
		case "application/vnd.google-apps.document":
			// Native Google Doc — use Docs API
			docsSvc, err := newDocsService(ctx, getClient, email)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			doc, err := docsSvc.Documents.Get(documentID).IncludeTabsContent(true).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
			}

			bodyText = extractDocText(doc)
		default:
			// Non-native file — download via Drive and try UTF-8 decode
			resp, err := driveSvc.Files.Get(documentID).SupportsAllDrives(true).Download()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Drive API download error: %v", err)), nil
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("reading download: %v", err)), nil
			}
			bodyText = tryDecodeUTF8(data, mimeType)
		}

		header := fmt.Sprintf("File: \"%s\" (ID: %s, Type: %s)\nLink: %s\n\n--- CONTENT ---\n",
			fileName, documentID, mimeType, webViewLink)

		return mcp.NewToolResultText(header + bodyText), nil
	}
}

// extractDocText extracts readable text from a Google Doc, including tabs.
func extractDocText(doc *docs.Document) string {
	var parts []string

	// Process main document body
	if doc.Body != nil {
		mainContent := extractTextFromElements(doc.Body.Content, "")
		if strings.TrimSpace(mainContent) != "" {
			parts = append(parts, mainContent)
		}
	}

	// Process all tabs
	for _, tab := range doc.Tabs {
		tabContent := processTabHierarchy(tab, 0)
		if strings.TrimSpace(tabContent) != "" {
			parts = append(parts, tabContent)
		}
	}

	return strings.Join(parts, "")
}

// processTabHierarchy recursively processes a tab and its child tabs.
func processTabHierarchy(tab *docs.Tab, level int) string {
	var b strings.Builder

	if tab.DocumentTab != nil {
		b.WriteString(processDocumentTab(tab, level))
	}

	// Process child tabs
	for _, child := range tab.ChildTabs {
		b.WriteString(processTabHierarchy(child, level+1))
	}

	return b.String()
}

func processDocumentTab(tab *docs.Tab, level int) string {
	tabTitle := "Untitled Tab"
	tabID := "Unknown ID"
	if tab.TabProperties != nil {
		if tab.TabProperties.Title != "" {
			tabTitle = tab.TabProperties.Title
		}
		if tab.TabProperties.TabId != "" {
			tabID = tab.TabProperties.TabId
		}
	}
	if level > 0 {
		tabTitle = strings.Repeat("    ", level) + tabTitle + " ( ID: " + tabID + ")"
	}
	if tab.DocumentTab.Body == nil {
		return ""
	}
	return extractTextFromElements(tab.DocumentTab.Body.Content, tabTitle)
}

// extractTextFromElements extracts text from document structural elements.
func extractTextFromElements(elements []*docs.StructuralElement, tabName string) string {
	return extractTextFromElementsWithDepth(elements, tabName, 0)
}

func extractTextFromElementsWithDepth(elements []*docs.StructuralElement, tabName string, depth int) string {
	if depth > 5 {
		return ""
	}

	var lines []string
	if tabName != "" {
		lines = append(lines, fmt.Sprintf("\n--- TAB: %s ---\n", tabName))
	}

	for _, elem := range elements {
		if elem.Paragraph != nil {
			var lineText string
			var lineTextSb282 strings.Builder
			for _, pe := range elem.Paragraph.Elements {
				if pe.TextRun != nil && pe.TextRun.Content != "" {
					lineTextSb282.WriteString(pe.TextRun.Content)
				}
			}
			lineText += lineTextSb282.String()
			if strings.TrimSpace(lineText) != "" {
				lines = append(lines, lineText)
			}
		} else if elem.Table != nil {
			for _, row := range elem.Table.TableRows {
				for _, cell := range row.TableCells {
					cellText := extractTextFromElementsWithDepth(cell.Content, "", depth+1)
					if strings.TrimSpace(cellText) != "" {
						lines = append(lines, cellText)
					}
				}
			}
		}
	}

	return strings.Join(lines, "")
}

// --- docs_list_in_folder ---

func registerListDocsInFolder(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_list_in_folder",
		mcp.WithDescription("List Google Docs in a Drive folder_id (document ids + names). Use for 'docs in this folder'. Not title search — use docs_search."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("folder_id", mcp.Description("The ID of the folder to list docs from. Defaults to 'root'.")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of documents to return. Defaults to 100.")),
	)
	s.AddTool(tool, handleListDocsInFolder(getClient))
}

func handleListDocsInFolder(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		folderID := optionalFolderID(request, "root")
		pageSize := request.GetInt("page_size", 100)

		driveSvc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		q := fmt.Sprintf("'%s' in parents and mimeType='application/vnd.google-apps.document' and trashed=false", folderID)

		resp, err := driveSvc.Files.List().
			Q(q).
			PageSize(int64(pageSize)).
			Fields("files(id, name, modifiedTime, webViewLink)").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		items := resp.Files
		if len(items) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No Google Docs found in folder '%s'.", folderID)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d Docs in folder '%s':", len(items), folderID)
		for _, f := range items {
			modified := f.ModifiedTime
			if modified == "" {
				modified = "N/A"
			}
			link := f.WebViewLink
			if link == "" {
				link = "#"
			}
			fmt.Fprintf(&b, "\n- %s (ID: %s) Modified: %s Link: %s", f.Name, f.Id, modified, link)
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- docs_create ---

func registerCreateDoc(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_create",
		mcp.WithDescription("Create a Google Doc. Required: title. Returns document_id. Then docs_modify_text / docs_get_content."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Document title.")),
		mcp.WithString("content", mcp.Description("Optional initial body text.")),
	)
	s.AddTool(tool, handleCreateDoc(getClient))
}

func handleCreateDoc(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "docs_create(title=…)"), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return needArg("title", "docs_create(title=…)"), nil
		}
		content := request.GetString("content", "")

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		doc, err := docsSvc.Documents.Create(&docs.Document{Title: title}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		docID := doc.DocumentId

		// Insert initial content if provided.
		if content != "" {
			req := &docs.BatchUpdateDocumentRequest{
				Requests: []*docs.Request{
					{
						InsertText: &docs.InsertTextRequest{
							Location: &docs.Location{Index: 1},
							Text:     content,
						},
					},
				},
			}
			_, err = docsSvc.Documents.BatchUpdate(docID, req).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Docs API error inserting content: %v", err)), nil
			}
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", docID)
		msg := fmt.Sprintf("Created Google Doc '%s' (ID: %s) for %s. Link: %s", title, docID, email, link)
		return mcp.NewToolResultText(msg), nil
	}
}

// --- docs_inspect_structure ---

func registerInspectDocStructure(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_inspect_structure",
		mcp.WithDescription("Doc layout stats: total_length, indices, tables (safe insert index). Use BEFORE docs_create_table_with_data or docs_insert_elements — pick index < total_length. Not full text — use docs_get_content."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to inspect.")),
		mcp.WithBoolean("detailed", mcp.Description("Whether to return detailed structure information. Defaults to false.")),
	)
	s.AddTool(tool, handleInspectDocStructure(getClient))
}

func handleInspectDocStructure(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		detailed := getBool(request, "detailed", false)

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		doc, err := docsSvc.Documents.Get(documentID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		var result map[string]any

		if detailed {
			result = buildDetailedStructure(doc)
		} else {
			result = buildBasicStructure(doc)
		}

		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON encoding error: %v", err)), nil
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		output := fmt.Sprintf("Document structure analysis for %s:\n\n%s\n\nLink: %s", documentID, string(jsonBytes), link)
		return mcp.NewToolResultText(output), nil
	}
}

// buildBasicStructure builds a basic analysis of document structure.
func buildBasicStructure(doc *docs.Document) map[string]any {
	elements := doc.Body.Content
	totalLength := int64(1)
	tableCount := 0
	paragraphCount := 0

	for _, elem := range elements {
		if elem.EndIndex > totalLength {
			totalLength = elem.EndIndex
		}
		if elem.Table != nil {
			tableCount++
		}
		if elem.Paragraph != nil {
			paragraphCount++
		}
	}

	result := map[string]any{
		"title":          doc.Title,
		"total_elements": len(elements),
		"total_length":   totalLength,
		"tables":         tableCount,
		"paragraphs":     paragraphCount,
	}

	// Add table details
	if tableCount > 0 {
		var tableDetails []map[string]any
		tableIdx := 0
		for _, elem := range elements {
			if elem.Table != nil {
				rows := int64(len(elem.Table.TableRows))
				cols := int64(0)
				if rows > 0 && len(elem.Table.TableRows[0].TableCells) > 0 {
					cols = int64(len(elem.Table.TableRows[0].TableCells))
				}
				tableDetails = append(tableDetails, map[string]any{
					"index":       tableIdx,
					"rows":        rows,
					"columns":     cols,
					"start_index": elem.StartIndex,
					"end_index":   elem.EndIndex,
				})
				tableIdx++
			}
		}
		result["table_details"] = tableDetails
	}

	return result
}

// buildDetailedStructure builds a detailed analysis of document structure.
func buildDetailedStructure(doc *docs.Document) map[string]any {
	elements := doc.Body.Content
	totalLength := int64(1)
	tableCount := 0
	paragraphCount := 0

	for _, elem := range elements {
		if elem.EndIndex > totalLength {
			totalLength = elem.EndIndex
		}
		if elem.Table != nil {
			tableCount++
		}
		if elem.Paragraph != nil {
			paragraphCount++
		}
	}

	hasHeaders := len(doc.Headers) > 0
	hasFooters := len(doc.Footers) > 0

	result := map[string]any{
		"title":        doc.Title,
		"total_length": totalLength,
		"statistics": map[string]any{
			"elements":    len(elements),
			"tables":      tableCount,
			"paragraphs":  paragraphCount,
			"has_headers": hasHeaders,
			"has_footers": hasFooters,
		},
		"elements": []map[string]any{},
	}

	elemSummaries := make([]map[string]any, 0, len(elements))
	var tableDetails []map[string]any
	tableIdx := 0

	for _, elem := range elements {
		summary := map[string]any{
			"start_index": elem.StartIndex,
			"end_index":   elem.EndIndex,
		}

		switch {
		case elem.Table != nil:
			summary["type"] = "table"
			rows := int64(len(elem.Table.TableRows))
			cols := int64(0)
			if rows > 0 && len(elem.Table.TableRows[0].TableCells) > 0 {
				cols = int64(len(elem.Table.TableRows[0].TableCells))
			}
			summary["rows"] = rows
			summary["columns"] = cols

			// Count cells
			cellCount := 0
			for _, row := range elem.Table.TableRows {
				cellCount += len(row.TableCells)
			}
			summary["cell_count"] = cellCount

			// Collect table detail for preview
			tableDetail := map[string]any{
				"index": tableIdx,
				"position": map[string]any{
					"start": elem.StartIndex,
					"end":   elem.EndIndex,
				},
				"dimensions": map[string]any{
					"rows":    rows,
					"columns": cols,
				},
			}

			// Extract first 3 rows as preview
			var preview [][]string
			maxRows := min(3, int(rows))
			for r := range maxRows {
				var rowData []string
				for _, cell := range elem.Table.TableRows[r].TableCells {
					cellText := extractCellText(cell)
					rowData = append(rowData, cellText)
				}
				preview = append(preview, rowData)
			}
			tableDetail["preview"] = preview
			tableDetails = append(tableDetails, tableDetail)
			tableIdx++

		case elem.Paragraph != nil:
			summary["type"] = "paragraph"
			// Text preview (first 100 chars)
			var textContentSb strings.Builder
			for _, pe := range elem.Paragraph.Elements {
				if pe.TextRun != nil && pe.TextRun.Content != "" {
					textContentSb.WriteString(pe.TextRun.Content)
				}
			}
			textContent := textContentSb.String()
			if len(textContent) > 100 {
				textContent = textContent[:100]
			}
			summary["text_preview"] = textContent
		case elem.SectionBreak != nil:
			summary["type"] = "section_break"
		case elem.TableOfContents != nil:
			summary["type"] = "table_of_contents"
		default:
			summary["type"] = "unknown"
		}

		elemSummaries = append(elemSummaries, summary)
	}

	result["elements"] = elemSummaries
	if len(tableDetails) > 0 {
		result["tables"] = tableDetails
	}

	return result
}

// extractCellText extracts text content from a table cell.
func extractCellText(cell *docs.TableCell) string {
	var text strings.Builder
	for _, elem := range cell.Content {
		if elem.Paragraph == nil {
			continue
		}
		for _, pe := range elem.Paragraph.Elements {
			if pe.TextRun != nil && pe.TextRun.Content != "" {
				text.WriteString(pe.TextRun.Content)
			}
		}
	}
	return strings.TrimSpace(text.String())
}

// --- docs_debug_table_structure ---

func registerDebugTableStructure(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_debug_table_structure",
		mcp.WithDescription("Debug table grid: rows/cols, cell indices, content. Use when populate failed or 'table not found'. Before populate_existing flows. Not general layout — use docs_inspect_structure."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document containing the table.")),
		mcp.WithNumber("table_index", mcp.Description("Which table to debug (0 = first table, 1 = second table, etc.). Defaults to 0.")),
	)
	s.AddTool(tool, handleDebugTableStructure(getClient))
}

func handleDebugTableStructure(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		tableIndex := request.GetInt("table_index", 0)

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		doc, err := docsSvc.Documents.Get(documentID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		// Find all tables in the document.
		var tables []*docs.StructuralElement
		for _, elem := range doc.Body.Content {
			if elem.Table != nil {
				tables = append(tables, elem)
			}
		}

		if tableIndex >= len(tables) {
			return mcp.NewToolResultText(fmt.Sprintf("Error: Table index %d not found. Document has %d table(s).", tableIndex, len(tables))), nil
		}

		tableElem := tables[tableIndex]
		table := tableElem.Table
		rows := len(table.TableRows)
		cols := 0
		if rows > 0 {
			cols = len(table.TableRows[0].TableCells)
		}

		debugInfo := map[string]any{
			"table_index": tableIndex,
			"dimensions":  fmt.Sprintf("%dx%d", rows, cols),
			"table_range": fmt.Sprintf("[%d-%d]", tableElem.StartIndex, tableElem.EndIndex),
			"cells":       []any{},
		}

		var allRows []any
		for rowIdx, row := range table.TableRows {
			var rowInfo []map[string]any
			for colIdx, cell := range row.TableCells {
				cellContent := extractCellText(cell)
				contentElements := 0
				for _, elem := range cell.Content {
					contentElements++
					_ = elem
				}

				insertionIndex := int64(0)
				if len(cell.Content) > 0 {
					insertionIndex = cell.Content[0].StartIndex
				}

				cellDebug := map[string]any{
					"position":               fmt.Sprintf("(%d,%d)", rowIdx, colIdx),
					"range":                  fmt.Sprintf("[%d-%d]", cell.StartIndex, cell.EndIndex),
					"insertion_index":        insertionIndex,
					"current_content":        fmt.Sprintf("%q", cellContent),
					"content_elements_count": contentElements,
				}
				rowInfo = append(rowInfo, cellDebug)
			}
			allRows = append(allRows, rowInfo)
		}
		debugInfo["cells"] = allRows

		jsonBytes, err := json.MarshalIndent(debugInfo, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON encoding error: %v", err)), nil
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		output := fmt.Sprintf("Table structure debug for table %d:\n\n%s\n\nLink: %s", tableIndex, string(jsonBytes), link)
		return mcp.NewToolResultText(output), nil
	}
}

// =====================================================================
// Docs Write Tools (US-014)
// =====================================================================

// --- docs_modify_text ---

func registerModifyDocText(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_modify_text",
		mcp.WithDescription("Insert or replace text at start_index in document_id. Required: document_id, start_index, text. Replace range: also end_index. Bold/colors → docs_batch_update. Global replace → docs_find_and_replace."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("Document id from docs_create / docs_get_content.")),
		mcp.WithNumber("start_index", mcp.Required(), mcp.Description("Insert/replace start index (use 1 near top of body).")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to insert, or replacement text when end_index is set.")),
		mcp.WithNumber("end_index", mcp.Description("If set with text, replaces [start_index, end_index).")),
	)
	s.AddTool(tool, handleModifyDocText(getClient))
}

func handleModifyDocText(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "docs_modify_text(document_id, start_index, text=…)"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "docs_create(title=…) then docs_modify_text(document_id, start_index, text=…)"), nil
		}
		startIndex := request.GetInt("start_index", 0)
		endIndex := request.GetInt("end_index", -1)
		text := request.GetString("text", "")

		// Formatting still accepted if a client sends it (not in lean schema).
		args := request.GetArguments()
		_, hasBold := args["bold"]
		_, hasItalic := args["italic"]
		_, hasUnderline := args["underline"]
		fontSize := request.GetInt("font_size", 0)
		fontFamily := request.GetString("font_family", "")
		textColor := request.GetString("text_color", "")
		bgColor := request.GetString("background_color", "")

		hasFormatting := hasBold || hasItalic || hasUnderline || fontSize > 0 || fontFamily != "" || textColor != "" || bgColor != ""

		if text == "" && !hasFormatting {
			return needArg("text", "docs_modify_text(document_id, start_index, text=…). For bold/colors use docs_batch_update"), nil
		}

		if hasFormatting && endIndex < 0 {
			return needArg("end_index", "docs_modify_text(…, end_index=…) for formatting, or prefer docs_batch_update"), nil
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var requests []*docs.Request
		var operations []string

		// Handle text insertion/replacement.
		switch text {
		case "":
		default:
			if endIndex > startIndex {
				// Text replacement
				if startIndex == 0 {
					requests = append(requests, &docs.Request{
						InsertText: &docs.InsertTextRequest{
							Location: &docs.Location{Index: 1},
							Text:     text,
						},
					})
					adjustedEnd := int64(endIndex) + int64(len(text))
					requests = append(requests, &docs.Request{
						DeleteContentRange: &docs.DeleteContentRangeRequest{
							Range: &docs.Range{
								StartIndex: 1 + int64(len(text)),
								EndIndex:   adjustedEnd,
							},
						},
					})
				} else {
					requests = append(requests,
						&docs.Request{
							DeleteContentRange: &docs.DeleteContentRangeRequest{
								Range: &docs.Range{
									StartIndex: int64(startIndex),
									EndIndex:   int64(endIndex),
								},
							},
						},
						&docs.Request{
							InsertText: &docs.InsertTextRequest{
								Location: &docs.Location{Index: int64(startIndex)},
								Text:     text,
							},
						},
					)
				}
				operations = append(operations, fmt.Sprintf("Replaced text from index %d to %d", startIndex, endIndex))
			} else {
				// Text insertion
				actualIndex := int64(startIndex)
				if startIndex == 0 {
					actualIndex = 1
				}
				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: actualIndex},
						Text:     text,
					},
				})
				operations = append(operations, fmt.Sprintf("Inserted text at index %d", startIndex))
			}
		}

		// Handle formatting.
		switch hasFormatting {
		case false:
		default:
			formatStart := int64(startIndex)
			formatEnd := int64(endIndex)

			if text != "" {
				if endIndex > startIndex {
					formatEnd = int64(startIndex) + int64(len(text))
				} else {
					actualIndex := int64(startIndex)
					if startIndex == 0 {
						actualIndex = 1
					}
					formatStart = actualIndex
					formatEnd = actualIndex + int64(len(text))
				}
			}

			if formatStart == 0 {
				formatStart = 1
			}
			if formatEnd <= formatStart {
				formatEnd = formatStart + 1
			}

			textStyle, fields := buildTextStyle(args, hasBold, hasItalic, hasUnderline, fontSize, fontFamily, textColor, bgColor)

			switch len(fields) {
			case 0:
			default:
				requests = append(requests, &docs.Request{
					UpdateTextStyle: &docs.UpdateTextStyleRequest{
						Range: &docs.Range{
							StartIndex: formatStart,
							EndIndex:   formatEnd,
						},
						TextStyle: textStyle,
						Fields:    strings.Join(fields, ","),
					},
				})

				var formatDetails []string
				if hasBold {
					formatDetails = append(formatDetails, fmt.Sprintf("bold=%v", args["bold"]))
				}
				if hasItalic {
					formatDetails = append(formatDetails, fmt.Sprintf("italic=%v", args["italic"]))
				}
				if hasUnderline {
					formatDetails = append(formatDetails, fmt.Sprintf("underline=%v", args["underline"]))
				}
				if fontSize > 0 {
					formatDetails = append(formatDetails, fmt.Sprintf("font_size=%d", fontSize))
				}
				if fontFamily != "" {
					formatDetails = append(formatDetails, "font_family="+fontFamily)
				}
				if textColor != "" {
					formatDetails = append(formatDetails, "text_color="+textColor)
				}
				if bgColor != "" {
					formatDetails = append(formatDetails, "background_color="+bgColor)
				}
				operations = append(operations, fmt.Sprintf("Applied formatting (%s) to range %d-%d", strings.Join(formatDetails, ", "), formatStart, formatEnd))
			}
		}

		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		operationSummary := strings.Join(operations, "; ")
		textInfo := ""
		if text != "" {
			textInfo = fmt.Sprintf(" Text length: %d characters.", len(text))
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s in document %s.%s Link: %s", operationSummary, documentID, textInfo, link)), nil
	}
}

// buildTextStyle constructs a docs.TextStyle and field list from formatting parameters.
func buildTextStyle(args map[string]any, hasBold, hasItalic, hasUnderline bool, fontSize int, fontFamily, textColor, bgColor string) (*docs.TextStyle, []string) {
	style := &docs.TextStyle{}
	var fields []string

	if hasBold {
		b, _ := args["bold"].(bool)
		style.Bold = b
		if !b {
			style.ForceSendFields = append(style.ForceSendFields, "Bold")
		}
		fields = append(fields, "bold")
	}
	if hasItalic {
		b, _ := args["italic"].(bool)
		style.Italic = b
		if !b {
			style.ForceSendFields = append(style.ForceSendFields, "Italic")
		}
		fields = append(fields, "italic")
	}
	if hasUnderline {
		b, _ := args["underline"].(bool)
		style.Underline = b
		if !b {
			style.ForceSendFields = append(style.ForceSendFields, "Underline")
		}
		fields = append(fields, "underline")
	}
	if fontSize > 0 {
		style.FontSize = &docs.Dimension{Magnitude: float64(fontSize), Unit: "PT"}
		fields = append(fields, "fontSize")
	}
	if fontFamily != "" {
		style.WeightedFontFamily = &docs.WeightedFontFamily{FontFamily: fontFamily}
		fields = append(fields, "weightedFontFamily")
	}
	if textColor != "" {
		rgb := normalizeColor(textColor)
		if rgb != nil {
			style.ForegroundColor = &docs.OptionalColor{Color: &docs.Color{RgbColor: rgb}}
			fields = append(fields, "foregroundColor")
		}
	}
	if bgColor != "" {
		rgb := normalizeColor(bgColor)
		if rgb != nil {
			style.BackgroundColor = &docs.OptionalColor{Color: &docs.Color{RgbColor: rgb}}
			fields = append(fields, "backgroundColor")
		}
	}

	return style, fields
}

// normalizeColor converts "#RRGGBB" hex to docs.RgbColor.
func normalizeColor(hex string) *docs.RgbColor {
	if len(hex) != 7 || hex[0] != '#' {
		return nil
	}
	h := hex[1:]
	r, g, b := hexToDec(h[0:2]), hexToDec(h[2:4]), hexToDec(h[4:6])
	if r < 0 || g < 0 || b < 0 {
		return nil
	}
	return &docs.RgbColor{
		Red:             float64(r) / 255,
		Green:           float64(g) / 255,
		Blue:            float64(b) / 255,
		ForceSendFields: []string{"Red", "Green", "Blue"},
	}
}

func hexToDec(s string) int {
	v := 0
	for _, c := range s {
		v *= 16
		switch {
		case c >= '0' && c <= '9':
			v += int(c - '0')
		case c >= 'a' && c <= 'f':
			v += int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v += int(c-'A') + 10
		default:
			return -1
		}
	}
	return v
}

// --- docs_find_and_replace ---

func registerFindAndReplaceDoc(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_find_and_replace",
		mcp.WithDescription("Find/replace all occurrences of text in document_id. Use for 'change every X to Y'. Not for single indexed edit — use docs_modify_text."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithString("find_text", mcp.Required(), mcp.Description("Text to search for.")),
		mcp.WithString("replace_text", mcp.Required(), mcp.Description("Text to replace with.")),
		mcp.WithBoolean("match_case", mcp.Description("Whether to match case exactly. Defaults to false.")),
	)
	s.AddTool(tool, handleFindAndReplaceDoc(getClient))
}

func handleFindAndReplaceDoc(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		findText, err := request.RequireString("find_text")
		if err != nil {
			return mcp.NewToolResultError("find_text is required"), nil
		}
		replaceText, err := request.RequireString("replace_text")
		if err != nil {
			return mcp.NewToolResultError("replace_text is required"), nil
		}
		matchCase := getBool(request, "match_case", false)

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{
				{
					ReplaceAllText: &docs.ReplaceAllTextRequest{
						ContainsText: &docs.SubstringMatchCriteria{
							Text:            findText,
							MatchCase:       matchCase,
							ForceSendFields: []string{"MatchCase"},
						},
						ReplaceText: replaceText,
					},
				},
			},
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		replacements := int64(0)
		if len(resp.Replies) > 0 && resp.Replies[0].ReplaceAllText != nil {
			replacements = resp.Replies[0].ReplaceAllText.OccurrencesChanged
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Replaced %d occurrence(s) of '%s' with '%s' in document %s. Link: %s",
			replacements, findText, replaceText, documentID, link)), nil
	}
}

// --- docs_insert_elements ---

func registerInsertDocElements(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_insert_elements",
		mcp.WithDescription("Insert tables, lists, or page breaks at index in document_id. Use docs_inspect_structure for index. Tables with data: docs_create_table_with_data."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithString("element_type", mcp.Required(), mcp.Description("Type of element to insert ('table', 'list', 'page_break').")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("Position to insert element (0-based).")),
		mcp.WithNumber("rows", mcp.Description("Number of rows for table (required for table).")),
		mcp.WithNumber("columns", mcp.Description("Number of columns for table (required for table).")),
		mcp.WithString("list_type", mcp.Description("Type of list ('UNORDERED', 'ORDERED') (required for list).")),
		mcp.WithString("text", mcp.Description("Initial text content for list items.")),
	)
	s.AddTool(tool, handleInsertDocElements(getClient))
}

func handleInsertDocElements(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		elementType, err := request.RequireString("element_type")
		if err != nil {
			return mcp.NewToolResultError("element_type is required"), nil
		}
		index := int64(request.GetInt("index", 0))

		// Avoid first section break
		if index == 0 {
			index = 1
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var requests []*docs.Request
		var description string

		switch elementType {
		case "table":
			rows := request.GetInt("rows", 0)
			columns := request.GetInt("columns", 0)
			if rows == 0 || columns == 0 {
				return mcp.NewToolResultError("'rows' and 'columns' parameters are required for table insertion."), nil
			}
			requests = append(requests, &docs.Request{
				InsertTable: &docs.InsertTableRequest{
					Location: &docs.Location{Index: index},
					Rows:     int64(rows),
					Columns:  int64(columns),
				},
			})
			description = fmt.Sprintf("table (%dx%d)", rows, columns)

		case "list":
			listType := request.GetString("list_type", "")
			if listType == "" {
				return mcp.NewToolResultError("'list_type' parameter is required for list insertion ('UNORDERED' or 'ORDERED')."), nil
			}
			text := request.GetString("text", "List item")
			bulletPreset := "BULLET_DISC_CIRCLE_SQUARE"
			if listType == "ORDERED" {
				bulletPreset = "NUMBERED_DECIMAL_ALPHA_ROMAN"
			}
			requests = append(requests,
				&docs.Request{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: index},
						Text:     text + "\n",
					},
				},
				&docs.Request{
					CreateParagraphBullets: &docs.CreateParagraphBulletsRequest{
						Range: &docs.Range{
							StartIndex: index,
							EndIndex:   index + int64(len(text)),
						},
						BulletPreset: bulletPreset,
					},
				},
			)
			description = strings.ToLower(listType) + " list"

		case "page_break":
			requests = append(requests, &docs.Request{
				InsertPageBreak: &docs.InsertPageBreakRequest{
					Location: &docs.Location{Index: index},
				},
			})
			description = "page break"

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unsupported element type '%s'. Supported types: 'table', 'list', 'page_break'.", elementType)), nil
		}

		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Inserted %s at index %d in document %s. Link: %s", description, index, documentID, link)), nil
	}
}

// --- docs_insert_image ---

func registerInsertDocImage(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_insert_image",
		mcp.WithDescription("Insert image in document_id from Drive file_id or URL at index. Use for 'add logo/picture'. Not text — use docs_modify_text."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithString("image_source", mcp.Required(), mcp.Description("Drive file ID or public image URL.")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("Position to insert image (0-based).")),
		mcp.WithNumber("width", mcp.Description("Image width in points (optional).")),
		mcp.WithNumber("height", mcp.Description("Image height in points (optional).")),
	)
	s.AddTool(tool, handleInsertDocImage(getClient))
}

func handleInsertDocImage(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		imageSource, err := request.RequireString("image_source")
		if err != nil {
			return mcp.NewToolResultError("image_source is required"), nil
		}
		index := int64(request.GetInt("index", 0))
		width := request.GetInt("width", 0)
		height := request.GetInt("height", 0)

		if index == 0 {
			index = 1
		}

		// Determine if source is a Drive file ID or URL
		isDriveFile := !strings.HasPrefix(imageSource, "http://") && !strings.HasPrefix(imageSource, "https://")

		var imageURI string
		var sourceDescription string

		if isDriveFile {
			// Verify Drive file exists and is an image
			driveSvc, err := newDriveService(ctx, getClient, email)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fileMeta, err := driveSvc.Files.Get(imageSource).
				Fields("id, name, mimeType").
				SupportsAllDrives(true).
				Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Could not access Drive file %s: %v", imageSource, err)), nil
			}
			if !strings.HasPrefix(fileMeta.MimeType, "image/") {
				return mcp.NewToolResultError(fmt.Sprintf("File %s is not an image (MIME type: %s).", imageSource, fileMeta.MimeType)), nil
			}
			imageURI = "https://drive.google.com/uc?id=" + imageSource
			sourceDescription = "Drive file " + fileMeta.Name
		} else {
			imageURI = imageSource
			sourceDescription = "URL image"
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		insertReq := &docs.InsertInlineImageRequest{
			Location: &docs.Location{Index: index},
			Uri:      imageURI,
		}

		if width > 0 || height > 0 {
			objSize := &docs.Size{}
			if width > 0 {
				objSize.Width = &docs.Dimension{Magnitude: float64(width), Unit: "PT"}
			}
			if height > 0 {
				objSize.Height = &docs.Dimension{Magnitude: float64(height), Unit: "PT"}
			}
			insertReq.ObjectSize = objSize
		}

		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{{InsertInlineImage: insertReq}},
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		sizeInfo := ""
		if width > 0 || height > 0 {
			w, h := "auto", "auto"
			if width > 0 {
				w = strconv.Itoa(width)
			}
			if height > 0 {
				h = strconv.Itoa(height)
			}
			sizeInfo = fmt.Sprintf(" (size: %sx%s points)", w, h)
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Inserted %s%s at index %d in document %s. Link: %s",
			sourceDescription, sizeInfo, index, documentID, link)), nil
	}
}

// --- docs_update_headers_footers ---

func registerUpdateDocHeadersFooters(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_update_headers_footers",
		mcp.WithDescription("Update header/footer content in document_id. Use for 'fix the header'. Not body text — use docs_modify_text."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithString("section_type", mcp.Required(), mcp.Description("Type of section to update ('header' or 'footer').")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Text content for the header/footer.")),
		mcp.WithString("header_footer_type", mcp.Description("Type of header/footer ('DEFAULT', 'FIRST_PAGE_ONLY', 'EVEN_PAGE'). Defaults to 'DEFAULT'.")),
	)
	s.AddTool(tool, handleUpdateDocHeadersFooters(getClient))
}

func handleUpdateDocHeadersFooters(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		sectionType, err := request.RequireString("section_type")
		if err != nil {
			return mcp.NewToolResultError("section_type is required"), nil
		}
		content, err := request.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError("content is required"), nil
		}
		headerFooterType := request.GetString("header_footer_type", "DEFAULT")

		if sectionType != "header" && sectionType != "footer" {
			return mcp.NewToolResultError("section_type must be 'header' or 'footer'"), nil
		}
		validTypes := map[string]bool{"DEFAULT": true, "FIRST_PAGE_ONLY": true, "EVEN_PAGE": true}
		if !validTypes[headerFooterType] {
			return mcp.NewToolResultError("header_footer_type must be 'DEFAULT', 'FIRST_PAGE_ONLY', or 'EVEN_PAGE'"), nil
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Get document to find header/footer sections
		doc, err := docsSvc.Documents.Get(documentID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		// Find the target section
		var targetContent []*docs.StructuralElement

		if sectionType == "header" {
			if len(doc.Headers) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("No %s found in document. Please create a %s first in Google Docs.", sectionType, sectionType)), nil
			}
			for _, header := range doc.Headers {
				targetContent = header.Content
				break
			}
		} else {
			if len(doc.Footers) == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("No %s found in document. Please create a %s first in Google Docs.", sectionType, sectionType)), nil
			}
			for _, footer := range doc.Footers {
				targetContent = footer.Content
				break
			}
		}

		if len(targetContent) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Could not find content structure in %s to update", sectionType)), nil
		}

		// Find first paragraph element
		var startIdx, endIdx int64
		found := false
		for _, elem := range targetContent {
			if elem.Paragraph != nil {
				startIdx = elem.StartIndex
				endIdx = elem.EndIndex
				found = true
				break
			}
		}
		if !found {
			return mcp.NewToolResultError(fmt.Sprintf("Could not find content structure in %s to update", sectionType)), nil
		}

		var requests []*docs.Request
		// Delete existing content (keep paragraph end marker)
		if endIdx > startIdx {
			requests = append(requests, &docs.Request{
				DeleteContentRange: &docs.DeleteContentRangeRequest{
					Range: &docs.Range{
						StartIndex: startIdx,
						EndIndex:   endIdx - 1,
					},
				},
			})
		}
		// Insert new content
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: startIdx},
				Text:     content,
			},
		})

		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Updated %s content in document %s. Link: %s", sectionType, documentID, link)), nil
	}
}

// --- docs_batch_update ---

func registerBatchUpdateDoc(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_batch_update",
		mcp.WithDescription("Multi-step / formatting edits on document_id (insert, format, tables, find_replace). Simple text insert/replace → docs_modify_text."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithArray("operations", mcp.Required(), mcp.Description("List of operation dictionaries."), mcp.Items(map[string]any{"type": "object"})),
	)
	s.AddTool(tool, handleBatchUpdateDoc(getClient))
}

func handleBatchUpdateDoc(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}

		args := request.GetArguments()
		opsRaw, ok := args["operations"]
		if !ok {
			return mcp.NewToolResultError("operations is required"), nil
		}
		opsSlice, ok := opsRaw.([]any)
		if !ok {
			return mcp.NewToolResultError("operations must be an array"), nil
		}
		if len(opsSlice) == 0 {
			return mcp.NewToolResultError("No operations provided. Please provide at least one operation."), nil
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var requests []*docs.Request
		var descriptions []string

		for i, opRaw := range opsSlice {
			op, ok := opRaw.(map[string]any)
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("Operation %d: must be an object", i+1)), nil
			}

			opType, _ := op["type"].(string)
			if opType == "" {
				return mcp.NewToolResultError(fmt.Sprintf("Operation %d: missing 'type' field", i+1)), nil
			}

			reqs, desc, err := buildBatchOperationRequest(op, opType, i+1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			requests = append(requests, reqs...)
			descriptions = append(descriptions, desc)
		}

		resp, err := docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		repliesCount := 0
		if resp.Replies != nil {
			repliesCount = len(resp.Replies)
		}

		// Build summary
		summary := strings.Join(descriptions[:min(3, len(descriptions))], ", ")
		if len(descriptions) > 3 {
			remaining := len(descriptions) - 3
			summary += fmt.Sprintf(" and %d more operation", remaining)
			if remaining > 1 {
				summary += "s"
			}
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Successfully executed %d operations (%s) on document %s. API replies: %d. Link: %s",
			len(opsSlice), summary, documentID, repliesCount, link)), nil
	}
}

// buildBatchOperationRequest converts a single batch operation map into Docs API requests.
func buildBatchOperationRequest(op map[string]any, opType string, opNum int) ([]*docs.Request, string, error) {
	getInt := func(key string) (int64, error) {
		v, ok := op[key]
		if !ok {
			return 0, fmt.Errorf("operation %d (%s): missing required field '%s'", opNum, opType, key)
		}
		switch n := v.(type) {
		case float64:
			return int64(n), nil
		case int:
			return int64(n), nil
		default:
			return 0, fmt.Errorf("operation %d (%s): field '%s' must be a number", opNum, opType, key)
		}
	}

	getString := func(key string) (string, error) {
		v, ok := op[key]
		if !ok {
			return "", fmt.Errorf("operation %d (%s): missing required field '%s'", opNum, opType, key)
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("operation %d (%s): field '%s' must be a string", opNum, opType, key)
		}
		return s, nil
	}

	switch opType {
	case "insert_text":
		idx, err := getInt("index")
		if err != nil {
			return nil, "", err
		}
		text, err := getString("text")
		if err != nil {
			return nil, "", err
		}
		return []*docs.Request{{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: idx},
				Text:     text,
			},
		}}, fmt.Sprintf("insert text at %d", idx), nil

	case "delete_text":
		start, err := getInt("start_index")
		if err != nil {
			return nil, "", err
		}
		end, err := getInt("end_index")
		if err != nil {
			return nil, "", err
		}
		return []*docs.Request{{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{StartIndex: start, EndIndex: end},
			},
		}}, fmt.Sprintf("delete text %d-%d", start, end), nil

	case "replace_text":
		start, err := getInt("start_index")
		if err != nil {
			return nil, "", err
		}
		end, err := getInt("end_index")
		if err != nil {
			return nil, "", err
		}
		text, err := getString("text")
		if err != nil {
			return nil, "", err
		}
		preview := text
		if len(preview) > 20 {
			preview = preview[:20] + "..."
		}
		return []*docs.Request{
			{DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{StartIndex: start, EndIndex: end},
			}},
			{InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: start},
				Text:     text,
			}},
		}, fmt.Sprintf("replace text %d-%d with '%s'", start, end, preview), nil

	case "format_text":
		start, err := getInt("start_index")
		if err != nil {
			return nil, "", err
		}
		end, err := getInt("end_index")
		if err != nil {
			return nil, "", err
		}
		_, hasBold := op["bold"]
		_, hasItalic := op["italic"]
		_, hasUnderline := op["underline"]
		fs := 0
		if v, ok := op["font_size"]; ok {
			if n, ok := v.(float64); ok {
				fs = int(n)
			}
		}
		ff, _ := op["font_family"].(string)
		tc, _ := op["text_color"].(string)
		bc, _ := op["background_color"].(string)

		style, fields := buildTextStyle(op, hasBold, hasItalic, hasUnderline, fs, ff, tc, bc)
		if len(fields) == 0 {
			return nil, "", fmt.Errorf("operation %d (format_text): no formatting options provided", opNum)
		}

		return []*docs.Request{{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range:     &docs.Range{StartIndex: start, EndIndex: end},
				TextStyle: style,
				Fields:    strings.Join(fields, ","),
			},
		}}, fmt.Sprintf("format text %d-%d (%s)", start, end, strings.Join(fields, ", ")), nil

	case "insert_table":
		idx, err := getInt("index")
		if err != nil {
			return nil, "", err
		}
		rows, err := getInt("rows")
		if err != nil {
			return nil, "", err
		}
		cols, err := getInt("columns")
		if err != nil {
			return nil, "", err
		}
		return []*docs.Request{{
			InsertTable: &docs.InsertTableRequest{
				Location: &docs.Location{Index: idx},
				Rows:     rows,
				Columns:  cols,
			},
		}}, fmt.Sprintf("insert %dx%d table at %d", rows, cols, idx), nil

	case "insert_page_break":
		idx, err := getInt("index")
		if err != nil {
			return nil, "", err
		}
		return []*docs.Request{{
			InsertPageBreak: &docs.InsertPageBreakRequest{
				Location: &docs.Location{Index: idx},
			},
		}}, fmt.Sprintf("insert page break at %d", idx), nil

	case "find_replace":
		findText, err := getString("find_text")
		if err != nil {
			return nil, "", err
		}
		replaceText, err := getString("replace_text")
		if err != nil {
			return nil, "", err
		}
		matchCase := false
		if v, ok := op["match_case"]; ok {
			if b, ok := v.(bool); ok {
				matchCase = b
			}
		}
		return []*docs.Request{{
			ReplaceAllText: &docs.ReplaceAllTextRequest{
				ContainsText: &docs.SubstringMatchCriteria{
					Text:            findText,
					MatchCase:       matchCase,
					ForceSendFields: []string{"MatchCase"},
				},
				ReplaceText: replaceText,
			},
		}}, fmt.Sprintf("find/replace '%s' → '%s'", findText, replaceText), nil

	default:
		return nil, "", fmt.Errorf("operation %d: unsupported operation type '%s'. Supported: insert_text, delete_text, replace_text, format_text, insert_table, insert_page_break, find_replace", opNum, opType)
	}
}

// --- docs_create_table_with_data ---

func registerCreateTableWithData(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_create_table_with_data",
		mcp.WithDescription("Create table + fill cells in document_id at index (2D string array). MUST call docs_inspect_structure first; use index < total_length. Prefer over docs_insert_elements + manual fill."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the document to update.")),
		mcp.WithArray("table_data", mcp.Required(), mcp.Description("2D list of strings - EXACT format: [[\"col1\", \"col2\"], [\"row1col1\", \"row1col2\"]]"), mcp.Items(map[string]any{"type": "array", "items": map[string]any{"type": "string"}})),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("Document position (MANDATORY: get from docs_inspect_structure 'total_length').")),
		mcp.WithBoolean("bold_headers", mcp.Description("Whether to make first row bold (default: true).")),
	)
	s.AddTool(tool, handleCreateTableWithData(getClient))
}

func handleCreateTableWithData(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		index := int64(request.GetInt("index", 0))
		boldHeaders := getBool(request, "bold_headers", true)

		// Parse table_data from arguments
		args := request.GetArguments()
		tableDataRaw, ok := args["table_data"]
		if !ok {
			return mcp.NewToolResultError("table_data is required"), nil
		}

		tableData, err := parseTableData(tableDataRaw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("ERROR: %v", err)), nil
		}

		if len(tableData) == 0 {
			return mcp.NewToolResultError("ERROR: table_data must not be empty"), nil
		}
		cols := len(tableData[0])
		for i, row := range tableData {
			if len(row) != cols {
				return mcp.NewToolResultError(fmt.Sprintf("ERROR: Row %d has %d columns, expected %d. All rows must have the same number of columns.", i, len(row), cols)), nil
			}
		}

		rows := len(tableData)

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Step 1: Create empty table
		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{{
				InsertTable: &docs.InsertTableRequest{
					Location: &docs.Location{Index: index},
					Rows:     int64(rows),
					Columns:  int64(cols),
				},
			}},
		}).Do()
		if err != nil {
			// If index is at document boundary, retry with index-1
			if strings.Contains(err.Error(), "must be less than the end index") && index > 1 {
				index--
				_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
					Requests: []*docs.Request{{
						InsertTable: &docs.InsertTableRequest{
							Location: &docs.Location{Index: index},
							Rows:     int64(rows),
							Columns:  int64(cols),
						},
					}},
				}).Do()
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("ERROR: Table creation failed: %v", err)), nil
				}
			} else {
				return mcp.NewToolResultError(fmt.Sprintf("ERROR: Table creation failed: %v", err)), nil
			}
		}

		// Step 2: Populate cells one by one with fresh structure each time
		populationCount := 0
		for rowIdx, rowData := range tableData {
			for colIdx, cellText := range rowData {
				if cellText == "" {
					continue
				}

				// Get fresh document structure to find actual cell positions
				doc, err := docsSvc.Documents.Get(documentID).Do()
				if err != nil {
					continue
				}

				// Find the last table (the one we just created)
				var lastTable *docs.StructuralElement
				for _, elem := range doc.Body.Content {
					if elem.Table != nil {
						lastTable = elem
					}
				}
				if lastTable == nil || lastTable.Table == nil {
					continue
				}

				table := lastTable.Table
				if rowIdx >= len(table.TableRows) {
					continue
				}
				row := table.TableRows[rowIdx]
				if colIdx >= len(row.TableCells) {
					continue
				}
				cell := row.TableCells[colIdx]
				if len(cell.Content) == 0 {
					continue
				}

				insertionIdx := cell.Content[0].StartIndex

				insertReqs := []*docs.Request{{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: insertionIdx},
						Text:     cellText,
					},
				}}

				// Bold headers
				if boldHeaders && rowIdx == 0 {
					insertReqs = append(insertReqs, &docs.Request{
						UpdateTextStyle: &docs.UpdateTextStyleRequest{
							Range: &docs.Range{
								StartIndex: insertionIdx,
								EndIndex:   insertionIdx + int64(len(cellText)),
							},
							TextStyle: &docs.TextStyle{Bold: true},
							Fields:    "bold",
						},
					})
				}

				_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
					Requests: insertReqs,
				}).Do()
				if err == nil {
					populationCount++
				}
			}
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("SUCCESS: Successfully created %dx%d table and populated %d cells. Table: %dx%d, Index: %d. Link: %s",
			rows, cols, populationCount, rows, cols, index, link)), nil
	}
}

// parseTableData converts raw JSON table_data into [][]string.
func parseTableData(raw any) ([][]string, error) {
	rows, ok := raw.([]any)
	if !ok {
		return nil, errors.New("table_data must be a 2D array of strings")
	}
	var result [][]string
	for i, rowRaw := range rows {
		cols, ok := rowRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("table_data row %d must be an array", i)
		}
		var row []string
		for j, cell := range cols {
			s, ok := cell.(string)
			if !ok {
				// Try to convert to string
				s = fmt.Sprintf("%v", cell)
				_ = j
			}
			row = append(row, s)
		}
		result = append(result, row)
	}
	return result, nil
}

// --- docs_update_paragraph_style ---

func registerUpdateParagraphStyle(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_update_paragraph_style",
		mcp.WithDescription("Apply heading styles (H1–H6) and/or paragraph alignment/spacing on a range in document_id. Use for 'make this a heading'. Inline bold: docs_modify_text."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("Document ID to modify.")),
		mcp.WithNumber("start_index", mcp.Required(), mcp.Description("Start position (1-based).")),
		mcp.WithNumber("end_index", mcp.Required(), mcp.Description("End position (exclusive) - should cover the entire paragraph.")),
		mcp.WithNumber("heading_level", mcp.Description("Heading level 0-6 (0 = NORMAL_TEXT, 1 = H1, 2 = H2, etc.).")),
		mcp.WithString("alignment", mcp.Description("Text alignment - 'START' (left), 'CENTER', 'END' (right), or 'JUSTIFIED'.")),
		mcp.WithNumber("line_spacing", mcp.Description("Line spacing multiplier (1.0 = single, 1.5 = 1.5x, 2.0 = double).")),
		mcp.WithNumber("indent_first_line", mcp.Description("First line indent in points (e.g., 36 for 0.5 inch).")),
		mcp.WithNumber("indent_start", mcp.Description("Left/start indent in points.")),
		mcp.WithNumber("indent_end", mcp.Description("Right/end indent in points.")),
		mcp.WithNumber("space_above", mcp.Description("Space above paragraph in points (e.g., 12 for one line).")),
		mcp.WithNumber("space_below", mcp.Description("Space below paragraph in points.")),
	)
	s.AddTool(tool, handleUpdateParagraphStyle(getClient))
}

func handleUpdateParagraphStyle(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		startIndex := int64(request.GetInt("start_index", 0))
		endIndex := int64(request.GetInt("end_index", 0))

		if startIndex < 1 {
			return mcp.NewToolResultError("start_index must be >= 1"), nil
		}
		if endIndex <= startIndex {
			return mcp.NewToolResultError("end_index must be greater than start_index"), nil
		}

		args := request.GetArguments()
		paraStyle := &docs.ParagraphStyle{}
		var fields []string

		// Heading level
		if hlRaw, ok := args["heading_level"]; ok {
			hlFloat, ok := hlRaw.(float64)
			if !ok {
				return mcp.NewToolResultError("heading_level must be a number"), nil
			}
			hl := int(hlFloat)
			if hl < 0 || hl > 6 {
				return mcp.NewToolResultError("heading_level must be between 0 (normal text) and 6"), nil
			}
			if hl == 0 {
				paraStyle.NamedStyleType = "NORMAL_TEXT"
			} else {
				paraStyle.NamedStyleType = fmt.Sprintf("HEADING_%d", hl)
			}
			fields = append(fields, "namedStyleType")
		}

		// Alignment
		if alignRaw, ok := args["alignment"]; ok {
			alignStr, ok := alignRaw.(string)
			if !ok {
				return mcp.NewToolResultError("alignment must be a string"), nil
			}
			align := strings.ToUpper(alignStr)
			validAligns := map[string]bool{"START": true, "CENTER": true, "END": true, "JUSTIFIED": true}
			if !validAligns[align] {
				return mcp.NewToolResultError(fmt.Sprintf("Invalid alignment '%s'. Must be one of: START, CENTER, END, JUSTIFIED", align)), nil
			}
			paraStyle.Alignment = align
			fields = append(fields, "alignment")
		}

		// Line spacing
		if lsRaw, ok := args["line_spacing"]; ok {
			ls, ok := lsRaw.(float64)
			if !ok {
				return mcp.NewToolResultError("line_spacing must be a number"), nil
			}
			if ls <= 0 {
				return mcp.NewToolResultError("line_spacing must be positive"), nil
			}
			paraStyle.LineSpacing = ls * 100 // Convert to percentage
			fields = append(fields, "lineSpacing")
		}

		// Indentation / spacing (points)
		setDim := func(key, field string, set func(float64)) bool {
			v, ok := args[key]
			if !ok {
				return true
			}
			mag, ok := v.(float64)
			if !ok {
				return false
			}
			set(mag)
			fields = append(fields, field)
			return true
		}
		if !setDim("indent_first_line", "indentFirstLine", func(m float64) {
			paraStyle.IndentFirstLine = &docs.Dimension{Magnitude: m, Unit: "PT"}
		}) {
			return mcp.NewToolResultError("indent_first_line must be a number"), nil
		}
		if !setDim("indent_start", "indentStart", func(m float64) {
			paraStyle.IndentStart = &docs.Dimension{Magnitude: m, Unit: "PT"}
		}) {
			return mcp.NewToolResultError("indent_start must be a number"), nil
		}
		if !setDim("indent_end", "indentEnd", func(m float64) {
			paraStyle.IndentEnd = &docs.Dimension{Magnitude: m, Unit: "PT"}
		}) {
			return mcp.NewToolResultError("indent_end must be a number"), nil
		}
		if !setDim("space_above", "spaceAbove", func(m float64) {
			paraStyle.SpaceAbove = &docs.Dimension{Magnitude: m, Unit: "PT"}
		}) {
			return mcp.NewToolResultError("space_above must be a number"), nil
		}
		if !setDim("space_below", "spaceBelow", func(m float64) {
			paraStyle.SpaceBelow = &docs.Dimension{Magnitude: m, Unit: "PT"}
		}) {
			return mcp.NewToolResultError("space_below must be a number"), nil
		}

		if len(fields) == 0 {
			return mcp.NewToolResultText("No paragraph style changes specified for document " + documentID), nil
		}

		docsSvc, err := newDocsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		_, err = docsSvc.Documents.BatchUpdate(documentID, &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{{
				UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
					Range: &docs.Range{
						StartIndex: startIndex,
						EndIndex:   endIndex,
					},
					ParagraphStyle: paraStyle,
					Fields:         strings.Join(fields, ","),
				},
			}},
		}).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Docs API error: %v", err)), nil
		}

		// Build summary
		var summaryParts []string
		if paraStyle.NamedStyleType != "" {
			summaryParts = append(summaryParts, paraStyle.NamedStyleType)
		}
		var formatFields []string
		for _, f := range fields {
			if f != "namedStyleType" {
				formatFields = append(formatFields, f)
			}
		}
		if len(formatFields) > 0 {
			summaryParts = append(summaryParts, strings.Join(formatFields, ", "))
		}

		link := fmt.Sprintf("https://docs.google.com/document/d/%s/edit", documentID)
		return mcp.NewToolResultText(fmt.Sprintf("Applied paragraph style (%s) to range %d-%d in document %s. Link: %s",
			strings.Join(summaryParts, ", "), startIndex, endIndex, documentID, link)), nil
	}
}

// =====================================================================
// Docs Read Tools (US-013) continued
// =====================================================================

// --- docs_export_to_pdf ---

func registerExportDocToPDF(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("docs_export_to_pdf",
		mcp.WithDescription("Export document_id to PDF saved in Drive. Use for 'save doc as PDF'. Not for editing body — use docs_modify_text. Export URL only — drive_get_file_download_url."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("document_id", mcp.Required(), mcp.Description("ID of the Google Doc to export.")),
		mcp.WithString("pdf_filename", mcp.Description("Name for the PDF file. If not provided, uses original name + \"_PDF\".")),
		mcp.WithString("folder_id", mcp.Description("Drive folder ID to save PDF in. If not provided, saves in root.")),
	)
	s.AddTool(tool, handleExportDocToPDF(getClient))
}

func handleExportDocToPDF(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		documentID, err := requireGoogleID(request, "document_id")
		if err != nil {
			return googleIDError(err, "document_id", "document_id=…"), nil
		}
		pdfFilename := request.GetString("pdf_filename", "")
		folderID := optionalFolderID(request, "")

		driveSvc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Get file metadata to validate it's a Google Doc.
		fileMeta, err := driveSvc.Files.Get(documentID).
			Fields("id, name, mimeType, webViewLink").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: Could not access document %s: %v", documentID, err)), nil
		}

		mimeType := fileMeta.MimeType
		originalName := fileMeta.Name
		if originalName == "" {
			originalName = "Unknown Document"
		}
		webViewLink := fileMeta.WebViewLink
		if webViewLink == "" {
			webViewLink = "#"
		}

		if mimeType != "application/vnd.google-apps.document" {
			return mcp.NewToolResultError(fmt.Sprintf("Error: File '%s' is not a Google Doc (MIME type: %s). Only native Google Docs can be exported to PDF.", originalName, mimeType)), nil
		}

		// Export the document as PDF.
		resp, err := driveSvc.Files.Export(documentID, "application/pdf").Download()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: Failed to export document to PDF: %v", err)), nil
		}
		defer resp.Body.Close()

		pdfContent, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: Failed to read PDF data: %v", err)), nil
		}
		pdfSize := len(pdfContent)

		// Determine PDF filename.
		if pdfFilename == "" {
			pdfFilename = originalName + "_PDF.pdf"
		} else if !strings.HasSuffix(pdfFilename, ".pdf") {
			pdfFilename += ".pdf"
		}

		// Upload PDF to Drive.
		uploadMeta := &drive.File{
			Name:     pdfFilename,
			MimeType: "application/pdf",
		}
		if folderID != "" {
			uploadMeta.Parents = []string{folderID}
		}

		uploaded, err := driveSvc.Files.Create(uploadMeta).
			Media(bytes.NewReader(pdfContent), googleapi.ContentType("application/pdf")).
			Fields("id, name, webViewLink, parents").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: Failed to upload PDF to Drive: %v. PDF was generated successfully (%d bytes) but could not be saved to Drive.", err, pdfSize)), nil
		}

		pdfWebLink := uploaded.WebViewLink
		if pdfWebLink == "" {
			pdfWebLink = "#"
		}

		folderInfo := ""
		if folderID != "" {
			folderInfo = " in folder " + folderID
		} else if len(uploaded.Parents) > 0 {
			folderInfo = " in folder " + uploaded.Parents[0]
		}

		msg := fmt.Sprintf("Successfully exported '%s' to PDF and saved to Drive as '%s' (ID: %s, %d bytes)%s. PDF: %s | Original: %s",
			originalName, pdfFilename, uploaded.Id, pdfSize, folderInfo, pdfWebLink, webViewLink)
		return mcp.NewToolResultText(msg), nil
	}
}
