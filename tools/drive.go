package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// driveQueryPatterns detects structured Drive queries (vs free text).
var driveQueryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b\w+\s*(=|!=|>|<)\s*['"].*?['"]`),
	regexp.MustCompile(`(?i)\b\w+\s*(=|!=|>|<)\s*\d+`),
	regexp.MustCompile(`(?i)\bcontains\b`),
	regexp.MustCompile(`(?i)\bin\s+parents\b`),
	regexp.MustCompile(`(?i)\bhas\s*\{`),
	regexp.MustCompile(`(?i)\btrashed\s*=\s*(true|false)\b`),
	regexp.MustCompile(`(?i)\bstarred\s*=\s*(true|false)\b`),
	regexp.MustCompile(`(?i)['"][^'"]+['"]\s+in\s+parents`),
	regexp.MustCompile(`(?i)\bfullText\s+contains\b`),
	regexp.MustCompile(`(?i)\bname\s*(=|contains)\b`),
	regexp.MustCompile(`(?i)\bmimeType\s*(=|!=)\b`),
}

const driveFileFields = "nextPageToken, files(id, name, mimeType, webViewLink, iconLink, modifiedTime, size)"

// RegisterDriveTools registers all Drive tools with the MCP server.
func RegisterDriveTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools (US-010)
	registerSearchDriveFiles(s, getClient)
	registerGetDriveFileContent(s, getClient)
	registerGetDriveFileDownloadURL(s, getClient)
	registerListDriveItems(s, getClient)
	registerGetDriveFilePermissions(s, getClient)
	registerCheckDriveFilePublicAccess(s, getClient)
	registerGetDriveShareableLink(s, getClient)

	// Write tools (US-011)
	registerCreateDriveFile(s, getClient)
	registerImportToGoogleDoc(s, getClient)
	registerUpdateDriveFile(s, getClient)
	registerCopyDriveFile(s, getClient)
	registerShareDriveFile(s, getClient)
	registerBatchShareDriveFile(s, getClient)
	registerUpdateDrivePermission(s, getClient)
	registerRemoveDrivePermission(s, getClient)
	registerTransferDriveOwnership(s, getClient)
}

// newDriveService creates a drive.Service for the given user email.
func newDriveService(ctx context.Context, getClient httpClientFunc, email string) (*drive.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := drive.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Drive service: %w", err)
	}
	return svc, nil
}

// isStructuredQuery checks if the query looks like a structured Drive API query.
func isStructuredQuery(query string) bool {
	for _, p := range driveQueryPatterns {
		if p.MatchString(query) {
			return true
		}
	}
	return false
}

// buildDriveListCall configures a Drive files.list call with common params.
func buildDriveListCall(svc *drive.Service, query string, pageSize int64, driveID string, includeAllDrives bool, corpora string) *drive.FilesListCall {
	call := svc.Files.List().
		Q(query).
		PageSize(pageSize).
		Fields(driveFileFields).
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(includeAllDrives)

	if driveID != "" {
		call = call.DriveId(driveID)
		if corpora != "" {
			call = call.Corpora(corpora)
		} else {
			call = call.Corpora("drive")
		}
	} else if corpora != "" {
		call = call.Corpora(corpora)
	}

	return call
}

// formatDriveFileList formats a list of Drive files for display.
func formatDriveFileList(files []*drive.File, header string) string {
	var b strings.Builder
	b.WriteString(header)
	for _, f := range files {
		sizeStr := ""
		if f.Size > 0 {
			sizeStr = fmt.Sprintf(", Size: %d", f.Size)
		}
		modified := f.ModifiedTime
		if modified == "" {
			modified = "N/A"
		}
		link := f.WebViewLink
		if link == "" {
			link = "#"
		}
		fmt.Fprintf(&b, "\n- Name: \"%s\" (ID: %s, Type: %s%s, Modified: %s) Link: %s",
			f.Name, f.Id, f.MimeType, sizeStr, modified, link)
	}
	return b.String()
}

// --- drive_search_files ---

func registerSearchDriveFiles(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_search_files",
		mcp.WithDescription("Find Drive files by title (falls back to fullText). Paste a share URL to resolve one file. Returns ids + names. Then drive_get_file_content / docs_get_content / sheets_read_values. Folder browse → drive_list_items."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("query", mcp.Required(), mcp.Description("File title text, share/edit URL, or Drive operators (name contains, mimeType=, …).")),
		mcp.WithNumber("page_size", mcp.Description("Max files. Default: 10.")),
	)
	s.AddTool(tool, handleSearchDriveFiles(getClient))
}

func handleSearchDriveFiles(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `drive_search_files(query="…")`), nil
		}
		query, err := request.RequireString("query")
		if err != nil {
			return needArg("query", `drive_search_files(query="Budget")`), nil
		}
		pageSize := request.GetInt("page_size", 10)
		driveID := request.GetString("drive_id", "")
		includeAllDrives := getBool(request, "include_items_from_all_drives", true)
		corpora := request.GetString("corpora", "")

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Human paste: share/edit URL → resolve that file directly.
		if looksLikeGoogleURL(query) {
			fileID := extractGoogleResourceID(query)
			if fileID == "" {
				return mcp.NewToolResultError("query looks like a URL but no Google file id was found"), nil
			}
			meta, err := svc.Files.Get(fileID).
				Fields("id, name, mimeType, webViewLink, iconLink, modifiedTime, size").
				SupportsAllDrives(true).
				Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
			}
			header := fmt.Sprintf("Resolved 1 file for %s from URL:", email)
			return mcp.NewToolResultText(formatDriveFileList([]*drive.File{meta}, header)), nil
		}

		resp, err := listDriveFilesForQuery(svc, query, int64(pageSize), driveID, includeAllDrives, corpora)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		if len(resp.Files) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No files found for '%s'.", query)), nil
		}

		header := fmt.Sprintf("Found %d files for %s matching '%s':", len(resp.Files), email, query)
		return mcp.NewToolResultText(formatDriveFileList(resp.Files, header)), nil
	}
}

// listDriveFilesForQuery runs a structured Drive query as-is, or free-text name search
// with fullText fallback.
func listDriveFilesForQuery(svc *drive.Service, query string, pageSize int64, driveID string, includeAllDrives bool, corpora string) (*drive.FileList, error) {
	if isStructuredQuery(query) {
		return buildDriveListCall(svc, query, pageSize, driveID, includeAllDrives, corpora).Do()
	}

	escaped := strings.ReplaceAll(query, "'", "\\'")
	nameQuery := fmt.Sprintf("name contains '%s' and trashed=false", escaped)
	resp, err := buildDriveListCall(svc, nameQuery, pageSize, driveID, includeAllDrives, corpora).Do()
	if err != nil || len(resp.Files) > 0 {
		return resp, err
	}

	fullTextQuery := fmt.Sprintf("fullText contains '%s' and trashed=false", escaped)
	return buildDriveListCall(svc, fullTextQuery, pageSize, driveID, includeAllDrives, corpora).Do()
}

// --- drive_get_file_content ---

func registerGetDriveFileContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_get_file_content",
		mcp.WithDescription("Read file by file_id (Docs→text, Sheets→CSV, Slides→text; else UTF-8). Use for 'what's in this file'. Download URL → drive_get_file_download_url. Edit Docs → docs_get_content."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("Drive file id or share/edit URL.")),
	)
	s.AddTool(tool, handleGetDriveFileContent(getClient))
}

func handleGetDriveFileContent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_get_file_content(file_id=…)"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", `drive_search_files(query="…") then drive_get_file_content(file_id)`), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Resolve shortcuts and get metadata.
		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		mimeType := meta.MimeType
		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown File"
		}

		// Determine export MIME type for Google native files.
		exportMIME := googleNativeExportMIME(mimeType)

		data, err := downloadDriveContent(svc, fileID, exportMIME)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Try to decode as UTF-8 text.
		bodyText := tryDecodeUTF8(data, mimeType)

		link := meta.WebViewLink
		if link == "" {
			link = "#"
		}

		header := fmt.Sprintf("File: \"%s\" (ID: %s, Type: %s)\nLink: %s\n\n--- CONTENT ---\n",
			fileName, fileID, mimeType, link)

		return mcp.NewToolResultText(header + bodyText), nil
	}
}

// --- drive_get_file_download_url ---

func registerGetDriveFileDownloadURL(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_get_file_download_url",
		mcp.WithDescription("Download/export bytes for file_id (optional export_format: pdf/docx/xlsx/csv/pptx). Use when user needs the file. Prefer drive_get_file_content to read text in chat."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("Drive file id from drive_search_files.")),
		mcp.WithString("export_format", mcp.Description("Native export: pdf, docx, xlsx, csv, pptx.")),
	)
	s.AddTool(tool, handleGetDriveFileDownloadURL(getClient))
}

func handleGetDriveFileDownloadURL(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_get_file_download_url(file_id=…)"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", `drive_search_files(query="…") then drive_get_file_download_url(file_id)`), nil
		}
		exportFormat := request.GetString("export_format", "")

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID
		mimeType := meta.MimeType
		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown File"
		}

		// Determine export MIME type for Google native files.
		exportMIME, outputMIME := resolveExportFormat(mimeType, exportFormat)

		data, err := downloadDriveContent(svc, fileID, exportMIME)
		if exportMIME == "" {
			outputMIME = mimeType
		}
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sizeBytes := len(data)
		sizeKB := float64(sizeBytes) / 1024

		var b strings.Builder
		b.WriteString("File downloaded successfully!\n")
		fmt.Fprintf(&b, "File: %s\n", fileName)
		fmt.Fprintf(&b, "File ID: %s\n", fileID)
		fmt.Fprintf(&b, "Size: %.1f KB (%d bytes)\n", sizeKB, sizeBytes)
		fmt.Fprintf(&b, "MIME Type: %s\n", outputMIME)

		if exportMIME != "" {
			fmt.Fprintf(&b, "\nNote: Google native file exported to %s format.", outputMIME)
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_list_items ---

func registerListDriveItems(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_list_items",
		mcp.WithDescription("Browse folder contents (optional folder_id; drive_id for shared drives). Use for 'what's in this folder'. Not for name search — use drive_search_files."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("folder_id", mcp.Description("The ID of the Google Drive folder. Defaults to 'root'.")),
		mcp.WithNumber("page_size", mcp.Description("The maximum number of items to return. Defaults to 100.")),
		mcp.WithString("drive_id", mcp.Description("ID of the shared drive.")),
		mcp.WithBoolean("include_items_from_all_drives", mcp.Description("Whether items from all accessible shared drives should be included. Defaults to True.")),
		mcp.WithString("corpora", mcp.Description("Corpus to query ('user', 'drive', 'allDrives').")),
	)
	s.AddTool(tool, handleListDriveItems(getClient))
}

func handleListDriveItems(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		folderID := optionalFolderID(request, "root")
		pageSize := request.GetInt("page_size", 100)
		driveID := request.GetString("drive_id", "")
		includeAllDrives := getBool(request, "include_items_from_all_drives", true)
		corpora := request.GetString("corpora", "")

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Resolve folder shortcuts.
		resolvedFolderID, err := resolveFolderID(svc, folderID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		query := fmt.Sprintf("'%s' in parents and trashed=false", resolvedFolderID)

		resp, err := buildDriveListCall(svc, query, int64(pageSize), driveID, includeAllDrives, corpora).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		if len(resp.Files) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No items found in folder '%s'.", folderID)), nil
		}

		header := fmt.Sprintf("Found %d items in folder '%s' for %s:", len(resp.Files), folderID, email)
		return mcp.NewToolResultText(formatDriveFileList(resp.Files, header)), nil
	}
}

// --- drive_get_file_permissions ---

func registerGetDriveFilePermissions(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_get_file_permissions",
		mcp.WithDescription("File metadata + sharing permissions for file_id. Use before drive_share_file or update/remove permission. Not for file body — use drive_get_file_content."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file to check permissions for.")),
	)
	s.AddTool(tool, handleGetDriveFilePermissions(getClient))
}

func handleGetDriveFilePermissions(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Resolve shortcuts.
		resolvedID, _, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		fileMeta, err := svc.Files.Get(fileID).
			Fields("id, name, mimeType, size, modifiedTime, owners, permissions(id, type, role, emailAddress, domain, expirationTime, permissionDetails), webViewLink, webContentLink, shared, sharingUser, viewersCanCopyContent").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "File: %s\n", fileMeta.Name)
		fmt.Fprintf(&b, "ID: %s\n", fileID)
		fmt.Fprintf(&b, "Type: %s\n", fileMeta.MimeType)
		if fileMeta.Size > 0 {
			fmt.Fprintf(&b, "Size: %d bytes\n", fileMeta.Size)
		} else {
			b.WriteString("Size: N/A\n")
		}
		modified := fileMeta.ModifiedTime
		if modified == "" {
			modified = "N/A"
		}
		fmt.Fprintf(&b, "Modified: %s\n", modified)

		b.WriteString("\nSharing Status:\n")
		fmt.Fprintf(&b, "  Shared: %v\n", fileMeta.Shared)

		if fileMeta.SharingUser != nil {
			name := fileMeta.SharingUser.DisplayName
			if name == "" {
				name = "Unknown"
			}
			addr := fileMeta.SharingUser.EmailAddress
			if addr == "" {
				addr = "Unknown"
			}
			fmt.Fprintf(&b, "  Shared by: %s (%s)\n", name, addr)
		}

		perms := fileMeta.Permissions
		if len(perms) > 0 {
			fmt.Fprintf(&b, "  Number of permissions: %d\n", len(perms))
			b.WriteString("  Permissions:\n")
			for _, p := range perms {
				fmt.Fprintf(&b, "    - %s\n", formatPermissionInfo(p))
			}
		} else {
			b.WriteString("  No additional permissions (private file)\n")
		}

		b.WriteString("\nURLs:\n")
		link := fileMeta.WebViewLink
		if link == "" {
			link = "N/A"
		}
		fmt.Fprintf(&b, "  View Link: %s\n", link)
		if fileMeta.WebContentLink != "" {
			fmt.Fprintf(&b, "  Direct Download Link: %s\n", fileMeta.WebContentLink)
		}

		hasPublic := checkPublicLinkPermission(perms)
		if hasPublic {
			b.WriteString("\nThis file is shared with 'Anyone with the link' - it can be inserted into Google Docs\n")
		} else {
			b.WriteString("\nThis file is NOT shared with 'Anyone with the link' - it cannot be inserted into Google Docs\n")
			b.WriteString("   To fix: Right-click the file in Google Drive > Share > Anyone with the link > Viewer\n")
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_check_file_public_access ---

func registerCheckDriveFilePublicAccess(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_check_file_public_access",
		mcp.WithDescription("Find a Drive file by name and report if link sharing is public. Use for 'is this doc public'. Prefer drive_search_files + drive_get_file_permissions for full control."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_name", mcp.Required(), mcp.Description("The name of the file to check.")),
	)
	s.AddTool(tool, handleCheckDriveFilePublicAccess(getClient))
}

func handleCheckDriveFilePublicAccess(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileName, err := request.RequireString("file_name")
		if err != nil {
			return mcp.NewToolResultError("file_name is required"), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		escaped := strings.ReplaceAll(fileName, "'", "\\'")
		query := fmt.Sprintf("name = '%s'", escaped)

		files, err := svc.Files.List().
			Q(query).
			PageSize(10).
			Fields("files(id, name, mimeType, webViewLink)").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		if len(files.Files) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No file found with name '%s'", fileName)), nil
		}

		var b strings.Builder
		if len(files.Files) > 1 {
			fmt.Fprintf(&b, "Found %d files with name '%s':\n", len(files.Files), fileName)
			for _, f := range files.Files {
				fmt.Fprintf(&b, "  - %s (ID: %s)\n", f.Name, f.Id)
			}
			b.WriteString("\nChecking the first file...\n\n")
		}

		// Check permissions for the first file.
		fileID := files.Files[0].Id
		resolvedID, _, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		fileMeta, err := svc.Files.Get(fileID).
			Fields("id, name, mimeType, permissions, webViewLink, webContentLink, shared").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fmt.Fprintf(&b, "File: %s\n", fileMeta.Name)
		fmt.Fprintf(&b, "ID: %s\n", fileID)
		fmt.Fprintf(&b, "Type: %s\n", fileMeta.MimeType)
		fmt.Fprintf(&b, "Shared: %v\n\n", fileMeta.Shared)

		hasPublic := checkPublicLinkPermission(fileMeta.Permissions)
		if hasPublic {
			b.WriteString("PUBLIC ACCESS ENABLED - This file can be inserted into Google Docs\n")
			fmt.Fprintf(&b, "Use with insert_doc_image_url: https://drive.google.com/uc?export=view&id=%s\n", fileID)
		} else {
			b.WriteString("NO PUBLIC ACCESS - Cannot insert into Google Docs\n")
			b.WriteString("Fix: Drive > Share > 'Anyone with the link' > 'Viewer'\n")
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_get_shareable_link ---

func registerGetDriveShareableLink(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_get_shareable_link",
		mcp.WithDescription("Get view/download link for file_id. Use after drive_search_files for the URL. Not for changing permissions — use drive_share_file."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("Drive file or folder id.")),
	)
	s.AddTool(tool, handleGetDriveShareableLink(getClient))
}

func handleGetDriveShareableLink(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_get_shareable_link(file_id=…)"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", `drive_search_files(query="…") then drive_get_shareable_link(file_id)`), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, _, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		fileMeta, err := svc.Files.Get(fileID).
			Fields("id, name, mimeType, webViewLink, webContentLink, shared, permissions(id, type, role, emailAddress, domain, expirationTime)").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "File: %s\n", fileMeta.Name)
		fmt.Fprintf(&b, "ID: %s\n", fileID)
		fmt.Fprintf(&b, "Type: %s\n", fileMeta.MimeType)
		fmt.Fprintf(&b, "Shared: %v\n", fileMeta.Shared)

		link := fileMeta.WebViewLink
		if link == "" {
			link = "N/A"
		}
		fmt.Fprintf(&b, "\nLinks:\n  View: %s\n", link)
		if fileMeta.WebContentLink != "" {
			fmt.Fprintf(&b, "  Download: %s\n", fileMeta.WebContentLink)
		}

		perms := fileMeta.Permissions
		if len(perms) > 0 {
			b.WriteString("\nCurrent permissions:\n")
			for _, p := range perms {
				fmt.Fprintf(&b, "  - %s\n", formatPermissionInfo(p))
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_create_file ---

func registerCreateDriveFile(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_create_file",
		mcp.WithDescription("Create a Drive file from content (returns file id + link). Use for 'save this to Drive'. MD/DOCX→Google Doc → drive_import_to_doc."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_name", mcp.Required(), mcp.Description("New file name.")),
		mcp.WithString("content", mcp.Description("File body text.")),
		mcp.WithString("folder_id", mcp.Description("Parent folder id. Default: root.")),
	)
	s.AddTool(tool, handleCreateDriveFile(getClient))
}

func handleCreateDriveFile(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_create_file(file_name, content=…)"), nil
		}
		fileName, err := request.RequireString("file_name")
		if err != nil {
			return needArg("file_name", "drive_create_file(file_name, content=…)"), nil
		}
		content := request.GetString("content", "")
		folderID := optionalFolderID(request, "root")
		mimeType := request.GetString("mime_type", "text/plain")
		fileURL := request.GetString("fileUrl", "")

		if content == "" && fileURL == "" {
			return toolHint("content is required", "drive_create_file(file_name, content=…)"), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedFolderID, err := resolveFolderID(svc, folderID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fileMeta := &drive.File{
			Name:     fileName,
			Parents:  []string{resolvedFolderID},
			MimeType: mimeType,
		}

		// For now, only support direct content (fileUrl support requires HTTP client).
		var reader io.Reader
		if content != "" {
			reader = bytes.NewReader([]byte(content))
		} else {
			return mcp.NewToolResultError("fileUrl support is not yet implemented in the Go server. Please use 'content' parameter instead."), nil
		}

		created, err := svc.Files.Create(fileMeta).
			Media(reader, googleapi.ContentType(mimeType)).
			Fields("id, name, webViewLink").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		link := created.WebViewLink
		if link == "" {
			link = "No link available"
		}

		msg := fmt.Sprintf("Successfully created file '%s' (ID: %s) in folder '%s' for %s. Link: %s",
			created.Name, created.Id, folderID, email, link)
		return mcp.NewToolResultText(msg), nil
	}
}

// --- drive_import_to_doc ---

// googleDocsImportFormats maps file extensions to MIME types for Docs conversion.
var googleDocsImportFormats = map[string]string{
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".txt":      "text/plain",
	".text":     "text/plain",
	".html":     "text/html",
	".htm":      "text/html",
	".docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".doc":      "application/msword",
	".rtf":      "application/rtf",
	".odt":      "application/vnd.oasis.opendocument.text",
}

const googleDocsMIMEType = "application/vnd.google-apps.document"

func registerImportToGoogleDoc(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_import_to_doc",
		mcp.WithDescription("Import content as a native Google Doc (returns document_id). Use for 'convert to Google Doc'. Then docs_get_content. Plain upload → drive_create_file."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_name", mcp.Required(), mcp.Description("Doc title (extension ignored / used for format hint).")),
		mcp.WithString("content", mcp.Description("Text body (md/txt/html).")),
		mcp.WithString("folder_id", mcp.Description("Parent folder id. Default: root.")),
	)
	s.AddTool(tool, handleImportToGoogleDoc(getClient))
}

func handleImportToGoogleDoc(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_import_to_doc(file_name, content=…)"), nil
		}
		fileName, err := request.RequireString("file_name")
		if err != nil {
			return needArg("file_name", "drive_import_to_doc(file_name, content=…)"), nil
		}
		content := request.GetString("content", "")
		filePath := request.GetString("file_path", "")
		fileURL := request.GetString("file_url", "")
		sourceFormat := request.GetString("source_format", "")
		folderID := optionalFolderID(request, "root")

		// Validate exactly one source provided.
		sourceCount := 0
		if content != "" {
			sourceCount++
		}
		if filePath != "" {
			sourceCount++
		}
		if fileURL != "" {
			sourceCount++
		}
		if sourceCount == 0 {
			return toolHint("content is required", "drive_import_to_doc(file_name, content=…)"), nil
		}
		if sourceCount > 1 {
			return toolHint("provide only one source", "drive_import_to_doc(file_name, content=…)"), nil
		}

		// Determine source MIME type.
		var sourceMIME string
		if sourceFormat != "" {
			key := "." + strings.ToLower(strings.TrimPrefix(sourceFormat, "."))
			mime, ok := googleDocsImportFormats[key]
			if !ok {
				return mcp.NewToolResultError(fmt.Sprintf("Unsupported source_format: '%s'", sourceFormat)), nil
			}
			sourceMIME = mime
		} else {
			sourceMIME = detectSourceFormat(fileName, content)
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedFolderID, err := resolveFolderID(svc, folderID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		// Strip extension from file name for the Doc title.
		docName := fileName
		if idx := strings.LastIndex(fileName, "."); idx > 0 {
			docName = fileName[:idx]
		}

		fileMeta := &drive.File{
			Name:     docName,
			Parents:  []string{resolvedFolderID},
			MimeType: googleDocsMIMEType,
		}

		var reader io.Reader
		if content != "" {
			reader = bytes.NewReader([]byte(content))
		} else {
			return mcp.NewToolResultError("file_path and file_url support are not yet implemented in the Go server. Please use 'content' parameter instead."), nil
		}

		created, err := svc.Files.Create(fileMeta).
			Media(reader, googleapi.ContentType(sourceMIME)).
			Fields("id, name, webViewLink, mimeType").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		link := created.WebViewLink
		if link == "" {
			link = "No link available"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully imported '%s' as Google Doc\n", docName)
		fmt.Fprintf(&b, "   Document ID: %s\n", created.Id)
		fmt.Fprintf(&b, "   Source format: %s\n", sourceMIME)
		fmt.Fprintf(&b, "   Folder: %s\n", folderID)
		fmt.Fprintf(&b, "   Link: %s", link)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_update_file ---

func registerUpdateDriveFile(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_update_file",
		mcp.WithDescription("Update Drive file metadata (name, description, parents) by file_id. Not for Doc/Sheet/Slide body edits — use Docs/Sheets/Slides tools."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file to update. Required.")),
		mcp.WithString("name", mcp.Description("New name for the file.")),
		mcp.WithString("description", mcp.Description("New description for the file.")),
		mcp.WithString("mime_type", mcp.Description("New MIME type (note: changing type may require content upload).")),
		mcp.WithString("add_parents", mcp.Description("Comma-separated folder IDs to add as parents.")),
		mcp.WithString("remove_parents", mcp.Description("Comma-separated folder IDs to remove from parents.")),
		mcp.WithBoolean("starred", mcp.Description("Whether to star/unstar the file.")),
		mcp.WithBoolean("trashed", mcp.Description("Whether to move file to/from trash.")),
		mcp.WithBoolean("writers_can_share", mcp.Description("Whether editors can share the file.")),
		mcp.WithBoolean("copy_requires_writer_permission", mcp.Description("Whether copying requires writer permission.")),
		mcp.WithObject("properties", mcp.Description("Custom key-value properties for the file.")),
	)
	s.AddTool(tool, handleUpdateDriveFile(getClient))
}

func handleUpdateDriveFile(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}

		args := request.GetArguments()

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, _, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		// Build the update body.
		updateBody := &drive.File{}
		hasUpdate := false

		if v, ok := args["name"]; ok && v != nil {
			if s, ok := v.(string); ok {
				updateBody.Name = s
				hasUpdate = true
			}
		}
		if v, ok := args["description"]; ok && v != nil {
			if s, ok := v.(string); ok {
				updateBody.Description = s
				hasUpdate = true
			}
		}
		if v, ok := args["mime_type"]; ok && v != nil {
			if s, ok := v.(string); ok {
				updateBody.MimeType = s
				hasUpdate = true
			}
		}
		if v, ok := args["starred"]; ok && v != nil {
			if b, ok := v.(bool); ok {
				updateBody.Starred = b
				hasUpdate = true
				if !b {
					updateBody.ForceSendFields = append(updateBody.ForceSendFields, "Starred")
				}
			}
		}
		if v, ok := args["trashed"]; ok && v != nil {
			if b, ok := v.(bool); ok {
				updateBody.Trashed = b
				hasUpdate = true
				if !b {
					updateBody.ForceSendFields = append(updateBody.ForceSendFields, "Trashed")
				}
			}
		}
		if v, ok := args["writers_can_share"]; ok && v != nil {
			if b, ok := v.(bool); ok {
				updateBody.WritersCanShare = b
				hasUpdate = true
				if !b {
					updateBody.ForceSendFields = append(updateBody.ForceSendFields, "WritersCanShare")
				}
			}
		}
		if v, ok := args["copy_requires_writer_permission"]; ok && v != nil {
			if b, ok := v.(bool); ok {
				updateBody.CopyRequiresWriterPermission = b
				hasUpdate = true
				if !b {
					updateBody.ForceSendFields = append(updateBody.ForceSendFields, "CopyRequiresWriterPermission")
				}
			}
		}
		if v, ok := args["properties"]; ok && v != nil {
			if props, ok := v.(map[string]any); ok {
				strProps := make(map[string]string, len(props))
				for k, val := range props {
					strProps[k] = fmt.Sprintf("%v", val)
				}
				updateBody.Properties = strProps
				hasUpdate = true
			}
		}

		addParents := request.GetString("add_parents", "")
		removeParents := request.GetString("remove_parents", "")

		if !hasUpdate && addParents == "" && removeParents == "" {
			return mcp.NewToolResultError("No updates specified."), nil
		}

		call := svc.Files.Update(fileID, updateBody).
			SupportsAllDrives(true).
			Fields("id, name, webViewLink")

		if addParents != "" {
			call = call.AddParents(addParents)
		}
		if removeParents != "" {
			call = call.RemoveParents(removeParents)
		}

		updated, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		link := updated.WebViewLink
		if link == "" {
			link = "#"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully updated file: %s\n", updated.Name)
		fmt.Fprintf(&b, "   File ID: %s\n", fileID)
		fmt.Fprintf(&b, "View file: %s", link)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_copy_file ---

func registerCopyDriveFile(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_copy_file",
		mcp.WithDescription("Duplicate a file by file_id. Returns new file id. Use for 'make a copy of…'. Not for in-place edits — use drive_update_file or app-specific tools."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file to copy. Required.")),
		mcp.WithString("new_name", mcp.Description("New name for the copied file. If not provided, uses \"Copy of [original name]\".")),
		mcp.WithString("parent_folder_id", mcp.Description("The ID of the folder where the copy should be created. Defaults to 'root' (My Drive).")),
	)
	s.AddTool(tool, handleCopyDriveFile(getClient))
}

func handleCopyDriveFile(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}
		newName := request.GetString("new_name", "")
		parentFolderID := request.GetString("parent_folder_id", "root")

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID
		originalName := meta.Name
		if originalName == "" {
			originalName = "Unknown File"
		}

		resolvedFolderID, err := resolveFolderID(svc, parentFolderID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		copyBody := &drive.File{}
		if newName != "" {
			copyBody.Name = newName
		} else {
			copyBody.Name = "Copy of " + originalName
		}
		if resolvedFolderID != "root" {
			copyBody.Parents = []string{resolvedFolderID}
		}

		copied, err := svc.Files.Copy(fileID, copyBody).
			SupportsAllDrives(true).
			Fields("id, name, webViewLink, mimeType, parents").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully copied '%s'\n", originalName)
		fmt.Fprintf(&b, "\nOriginal file ID: %s\n", fileID)
		fmt.Fprintf(&b, "New file ID: %s\n", copied.Id)
		fmt.Fprintf(&b, "New file name: %s\n", copied.Name)
		fmt.Fprintf(&b, "File type: %s\n", copied.MimeType)
		fmt.Fprintf(&b, "Location: %s\n", parentFolderID)
		link := copied.WebViewLink
		if link == "" {
			link = "N/A"
		}
		fmt.Fprintf(&b, "\nView copied file: %s", link)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_share_file ---

func registerShareDriveFile(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_share_file",
		mcp.WithDescription("Share file_id with one recipient (email + role). Confirm sensitive shares. Link-only → share_type=anyone. Many people → drive_batch_share_file. URL only → drive_get_shareable_link."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("Drive file or folder id.")),
		mcp.WithString("share_with", mcp.Description("Recipient email (or domain). Omit for share_type=anyone.")),
		mcp.WithString("role", mcp.Description("reader, commenter, or writer. Default: reader.")),
		mcp.WithString("share_type", mcp.Description("user, group, domain, or anyone. Default: user.")),
	)
	s.AddTool(tool, handleShareDriveFile(getClient))
}

func handleShareDriveFile(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "drive_share_file(file_id, share_with, role)"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", `drive_search_files(query="…") then drive_share_file(file_id, share_with, role)`), nil
		}
		shareWith := request.GetString("share_with", "")
		role := request.GetString("role", "reader")
		shareType := request.GetString("share_type", "user")
		sendNotification := getBool(request, "send_notification", true)
		emailMessage := request.GetString("email_message", "")
		expirationTime := request.GetString("expiration_time", "")

		args := request.GetArguments()

		// Validate role.
		if role != "reader" && role != "commenter" && role != "writer" {
			return toolHint(fmt.Sprintf("invalid role '%s'", role), `drive_share_file(file_id, share_with, role="reader"|"commenter"|"writer")`), nil
		}
		// Validate share_type.
		if shareType != "user" && shareType != "group" && shareType != "domain" && shareType != "anyone" {
			return toolHint(fmt.Sprintf("invalid share_type '%s'", shareType), `drive_share_file(file_id, share_with, share_type="user"|"anyone")`), nil
		}

		if (shareType == "user" || shareType == "group") && shareWith == "" {
			return needArg("share_with", `drive_share_file(file_id, share_with="user@example.com", role="reader")`), nil
		}
		if shareType == "domain" && shareWith == "" {
			return needArg("share_with", `drive_share_file(file_id, share_with="example.com", share_type="domain")`), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		perm := &drive.Permission{
			Type: shareType,
			Role: role,
		}
		switch shareType {
		case "user", "group":
			perm.EmailAddress = shareWith
		case "domain":
			perm.Domain = shareWith
		}
		if expirationTime != "" {
			perm.ExpirationTime = expirationTime
		}
		if shareType == "domain" || shareType == "anyone" {
			setDriveFileDiscovery(perm, args["allow_file_discovery"])
		}

		call := svc.Permissions.Create(fileID, perm).
			SupportsAllDrives(true).
			Fields("id, type, role, emailAddress, domain, expirationTime")

		if shareType == "user" || shareType == "group" {
			call = call.SendNotificationEmail(sendNotification)
			if emailMessage != "" {
				call = call.EmailMessage(emailMessage)
			}
		}

		created, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown"
		}
		link := meta.WebViewLink
		if link == "" {
			link = "N/A"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully shared '%s'\n", fileName)
		b.WriteString("\nPermission created:\n")
		fmt.Fprintf(&b, "  - %s\n", formatPermissionInfo(created))
		fmt.Fprintf(&b, "\nView link: %s", link)

		return mcp.NewToolResultText(b.String()), nil
	}
}

func downloadDriveContent(svc *drive.Service, fileID, exportMIME string) ([]byte, error) {
	if exportMIME != "" {
		resp, err := svc.Files.Export(fileID, exportMIME).Download()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", "Drive API export error", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading export: %w", err)
		}
		return data, nil
	}
	resp, err := svc.Files.Get(fileID).SupportsAllDrives(true).Download()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", "Drive API download error", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading download: %w", err)
	}
	return data, nil
}

func setDriveFileDiscovery(perm *drive.Permission, value any) {
	b, ok := value.(bool)
	if !ok {
		return
	}
	perm.AllowFileDiscovery = b
	if !b {
		perm.ForceSendFields = append(perm.ForceSendFields, "AllowFileDiscovery")
	}
}

// --- drive_batch_share_file ---

func registerBatchShareDriveFile(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_batch_share_file",
		mcp.WithDescription("Share file_id with multiple recipients (roles/expiry per person). Use for bulk sharing. Confirm when unclear. Prefer drive_share_file for one recipient."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file or folder to share. Required.")),
		mcp.WithArray("recipients", mcp.Required(), mcp.Description("List of recipient objects. Each should have: email (str), role (str, optional, default 'reader'), share_type (str, optional, default 'user'), expiration_time (str, optional). For domain shares, use 'domain' field instead of 'email'."), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithBoolean("send_notification", mcp.Description("Whether to send notification emails. Defaults to true.")),
		mcp.WithString("email_message", mcp.Description("Custom message for notification emails.")),
	)
	s.AddTool(tool, handleBatchShareDriveFile(getClient))
}

func handleBatchShareDriveFile(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}
		sendNotification := getBool(request, "send_notification", true)
		emailMessage := request.GetString("email_message", "")

		args := request.GetArguments()
		recipientsRaw, ok := args["recipients"]
		if !ok || recipientsRaw == nil {
			return mcp.NewToolResultError("recipients is required"), nil
		}
		recipientsList, ok := recipientsRaw.([]any)
		if !ok || len(recipientsList) == 0 {
			return mcp.NewToolResultError("recipients list cannot be empty"), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		var results []string
		successCount := 0
		failureCount := 0

		for _, recipientRaw := range recipientsList {
			recipient, ok := recipientRaw.(map[string]any)
			if !ok {
				results = append(results, "  - Skipped: invalid recipient format")
				failureCount++
				continue
			}

			shareType := "user"
			if v, ok := recipient["share_type"].(string); ok && v != "" {
				shareType = v
			}

			var identifier string
			if shareType == "domain" {
				domain, _ := recipient["domain"].(string)
				if domain == "" {
					results = append(results, "  - Skipped: missing domain for domain share")
					failureCount++
					continue
				}
				identifier = domain
			} else {
				recipientEmail, _ := recipient["email"].(string)
				if recipientEmail == "" {
					results = append(results, "  - Skipped: missing email address")
					failureCount++
					continue
				}
				identifier = recipientEmail
			}

			role := "reader"
			if v, ok := recipient["role"].(string); ok && v != "" {
				role = v
			}
			if role != "reader" && role != "commenter" && role != "writer" {
				results = append(results, fmt.Sprintf("  - %s: Failed - invalid role '%s'", identifier, role))
				failureCount++
				continue
			}

			perm := &drive.Permission{
				Type: shareType,
				Role: role,
			}
			if shareType == "domain" {
				perm.Domain = identifier
			} else {
				perm.EmailAddress = identifier
			}

			if v, ok := recipient["expiration_time"].(string); ok && v != "" {
				perm.ExpirationTime = v
			}

			call := svc.Permissions.Create(fileID, perm).
				SupportsAllDrives(true).
				Fields("id, type, role, emailAddress, domain, expirationTime")

			if shareType == "user" || shareType == "group" {
				call = call.SendNotificationEmail(sendNotification)
				if emailMessage != "" {
					call = call.EmailMessage(emailMessage)
				}
			}

			created, err := call.Do()
			if err != nil {
				results = append(results, fmt.Sprintf("  - %s: Failed - %v", identifier, err))
				failureCount++
			} else {
				results = append(results, "  - "+formatPermissionInfo(created))
				successCount++
			}
		}

		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown"
		}
		link := meta.WebViewLink
		if link == "" {
			link = "N/A"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Batch share results for '%s'\n", fileName)
		fmt.Fprintf(&b, "\nSummary: %d succeeded, %d failed\n", successCount, failureCount)
		b.WriteString("\nResults:\n")
		for _, r := range results {
			b.WriteString(r)
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "\nView link: %s", link)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_update_permission ---

func registerUpdateDrivePermission(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_update_permission",
		mcp.WithDescription("Change an existing permission on file_id. Required permission_id from drive_get_file_permissions. Not for new shares — use drive_share_file."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file or folder. Required.")),
		mcp.WithString("permission_id", mcp.Required(), mcp.Description("The ID of the permission to update (from drive_get_file_permissions). Required.")),
		mcp.WithString("role", mcp.Description("New role - 'reader', 'commenter', or 'writer'.")),
		mcp.WithString("expiration_time", mcp.Description("Expiration time in RFC 3339 format (e.g., \"2025-01-15T00:00:00Z\").")),
	)
	s.AddTool(tool, handleUpdateDrivePermission(getClient))
}

func handleUpdateDrivePermission(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}
		permissionID, err := request.RequireString("permission_id")
		if err != nil {
			return mcp.NewToolResultError("permission_id is required"), nil
		}
		role := request.GetString("role", "")
		expirationTime := request.GetString("expiration_time", "")

		if role == "" && expirationTime == "" {
			return mcp.NewToolResultError("Must provide at least one of: role, expiration_time"), nil
		}
		if role != "" && role != "reader" && role != "commenter" && role != "writer" {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid role '%s'. Must be 'reader', 'commenter', or 'writer'.", role)), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		// If role not provided, fetch current role (Google API requires it in update body).
		if role == "" {
			currentPerm, err := svc.Permissions.Get(fileID, permissionID).
				SupportsAllDrives(true).
				Fields("role").
				Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
			}
			role = currentPerm.Role
		}

		updateBody := &drive.Permission{
			Role: role,
		}
		if expirationTime != "" {
			updateBody.ExpirationTime = expirationTime
		}

		updated, err := svc.Permissions.Update(fileID, permissionID, updateBody).
			SupportsAllDrives(true).
			Fields("id, type, role, emailAddress, domain, expirationTime").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully updated permission on '%s'\n", fileName)
		b.WriteString("\nUpdated permission:\n")
		fmt.Fprintf(&b, "  - %s", formatPermissionInfo(updated))

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_remove_permission ---

func registerRemoveDrivePermission(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_remove_permission",
		mcp.WithDescription("Revoke access: remove permission_id on file_id. Destructive — confirm when unclear. List permissions with drive_get_file_permissions first."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file or folder. Required.")),
		mcp.WithString("permission_id", mcp.Required(), mcp.Description("The ID of the permission to remove (from drive_get_file_permissions). Required.")),
	)
	s.AddTool(tool, handleRemoveDrivePermission(getClient))
}

func handleRemoveDrivePermission(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}
		permissionID, err := request.RequireString("permission_id")
		if err != nil {
			return mcp.NewToolResultError("permission_id is required"), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		err = svc.Permissions.Delete(fileID, permissionID).
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully removed permission from '%s'\n", fileName)
		fmt.Fprintf(&b, "\nPermission ID '%s' has been revoked.", permissionID)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- drive_transfer_ownership ---

func registerTransferDriveOwnership(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("drive_transfer_ownership",
		mcp.WithDescription("Transfer file/folder ownership by file_id (irreversible; you become editor). Confirm always. Same-domain/personal rules apply. Not for sharing only — use drive_share_file."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
		mcp.WithString("file_id", mcp.Required(), mcp.Description("The ID of the file or folder to transfer. Required.")),
		mcp.WithString("new_owner_email", mcp.Required(), mcp.Description("Email address of the new owner. Required.")),
		mcp.WithBoolean("move_to_new_owners_root", mcp.Description("If true, moves the file to the new owner's My Drive root. Defaults to false.")),
	)
	s.AddTool(tool, handleTransferDriveOwnership(getClient))
}

func handleTransferDriveOwnership(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		fileID, err := requireGoogleID(request, "file_id")
		if err != nil {
			return googleIDError(err, "file_id", "file_id=…"), nil
		}
		newOwnerEmail, err := request.RequireString("new_owner_email")
		if err != nil {
			return mcp.NewToolResultError("new_owner_email is required"), nil
		}
		moveToRoot := getBool(request, "move_to_new_owners_root", false)

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resolvedID, meta, err := resolveDriveItem(svc, fileID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}
		fileID = resolvedID

		// Get current owners.
		fileMeta, err := svc.Files.Get(fileID).
			Fields("owners").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		var currentOwnerEmails []string
		if fileMeta.Owners != nil {
			for _, o := range fileMeta.Owners {
				if o.EmailAddress != "" {
					currentOwnerEmails = append(currentOwnerEmails, o.EmailAddress)
				}
			}
		}

		perm := &drive.Permission{
			Type:         "user",
			Role:         "owner",
			EmailAddress: newOwnerEmail,
		}

		_, err = svc.Permissions.Create(fileID, perm).
			TransferOwnership(true).
			MoveToNewOwnersRoot(moveToRoot).
			SupportsAllDrives(true).
			Fields("id, type, role, emailAddress").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Drive API error: %v", err)), nil
		}

		fileName := meta.Name
		if fileName == "" {
			fileName = "Unknown"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Successfully transferred ownership of '%s'\n", fileName)
		fmt.Fprintf(&b, "\nNew owner: %s\n", newOwnerEmail)
		if len(currentOwnerEmails) > 0 {
			fmt.Fprintf(&b, "Previous owner(s): %s\n", strings.Join(currentOwnerEmails, ", "))
		} else {
			b.WriteString("Previous owner(s): Unknown\n")
		}
		if moveToRoot {
			fmt.Fprintf(&b, "File moved to %s's My Drive root.\n", newOwnerEmail)
		}
		b.WriteString("\nNote: Previous owner now has editor access.")

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- Drive helper functions ---

const shortcutMIMEType = "application/vnd.google-apps.shortcut"

const folderMIMEType = "application/vnd.google-apps.folder"

// resolveDriveItem resolves Drive shortcuts to the real item.
// Returns the resolved file ID and file metadata.
func resolveDriveItem(svc *drive.Service, fileID string) (string, *drive.File, error) {
	fileID = extractGoogleResourceID(fileID)
	if fileID == "" {
		return "", nil, errors.New("invalid file id or share URL")
	}
	const maxDepth = 5
	currentID := fileID

	for depth := 0; ; depth++ {
		meta, err := svc.Files.Get(currentID).
			Fields("id, mimeType, name, webViewLink, parents, shortcutDetails(targetId, targetMimeType)").
			SupportsAllDrives(true).
			Do()
		if err != nil {
			return "", nil, err
		}

		if meta.MimeType != shortcutMIMEType {
			return currentID, meta, nil
		}

		if meta.ShortcutDetails == nil || meta.ShortcutDetails.TargetId == "" {
			return "", nil, fmt.Errorf("shortcut '%s' is missing target details", currentID)
		}

		if depth >= maxDepth {
			return "", nil, fmt.Errorf("shortcut resolution exceeded %d hops starting from '%s'", maxDepth, fileID)
		}
		currentID = meta.ShortcutDetails.TargetId
	}
}

// resolveFolderID resolves a folder ID that might be a shortcut, ensuring the result is a folder.
func resolveFolderID(svc *drive.Service, folderID string) (string, error) {
	folderID = extractGoogleResourceID(folderID)
	if folderID == "" {
		return "", errors.New("invalid folder id or share URL")
	}
	if folderID == "root" {
		return "root", nil
	}

	resolvedID, meta, err := resolveDriveItem(svc, folderID)
	if err != nil {
		return "", err
	}

	if meta.MimeType != folderMIMEType {
		return "", fmt.Errorf("resolved ID '%s' (from '%s') is not a folder; mimeType=%s", resolvedID, folderID, meta.MimeType)
	}

	return resolvedID, nil
}

// googleNativeExportMIME returns the export MIME type for Google native files.
func googleNativeExportMIME(mimeType string) string {
	switch mimeType {
	case "application/vnd.google-apps.document":
		return "text/plain"
	case "application/vnd.google-apps.spreadsheet":
		return "text/csv"
	case "application/vnd.google-apps.presentation":
		return "text/plain"
	default:
		return ""
	}
}

// resolveExportFormat determines the export MIME type based on file type and requested format.
// Returns (exportMIME, outputMIME).
func resolveExportFormat(mimeType, exportFormat string) (string, string) {
	switch mimeType {
	case "application/vnd.google-apps.document":
		if exportFormat == "docx" {
			m := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
			return m, m
		}
		return "application/pdf", "application/pdf"

	case "application/vnd.google-apps.spreadsheet":
		switch exportFormat {
		case "csv":
			return "text/csv", "text/csv"
		case "pdf":
			return "application/pdf", "application/pdf"
		default:
			m := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			return m, m
		}

	case "application/vnd.google-apps.presentation":
		if exportFormat == "pptx" {
			m := "application/vnd.openxmlformats-officedocument.presentationml.presentation"
			return m, m
		}
		return "application/pdf", "application/pdf"

	default:
		return "", mimeType
	}
}

// tryDecodeUTF8 attempts to decode bytes as UTF-8 text.
func tryDecodeUTF8(data []byte, mimeType string) string {
	// Check if the data is valid UTF-8 text.
	s := string(data)
	for _, r := range s {
		if r == '\uFFFD' && len(data) > 0 {
			return fmt.Sprintf("[Binary or unsupported text encoding for mimeType '%s' - %d bytes]", mimeType, len(data))
		}
	}
	return s
}

// checkPublicLinkPermission checks if any permission is "anyone" with read/write/comment access.
func checkPublicLinkPermission(perms []*drive.Permission) bool {
	for _, p := range perms {
		if p.Type == "anyone" && (p.Role == "reader" || p.Role == "writer" || p.Role == "commenter") {
			return true
		}
	}
	return false
}

// formatPermissionInfo formats a Drive permission for display.
func formatPermissionInfo(p *drive.Permission) string {
	var base string
	switch p.Type {
	case "anyone":
		base = fmt.Sprintf("Anyone with the link (%s) [id: %s]", p.Role, p.Id)
	case "user":
		email := p.EmailAddress
		if email == "" {
			email = "unknown"
		}
		base = fmt.Sprintf("User: %s (%s) [id: %s]", email, p.Role, p.Id)
	case "group":
		email := p.EmailAddress
		if email == "" {
			email = "unknown"
		}
		base = fmt.Sprintf("Group: %s (%s) [id: %s]", email, p.Role, p.Id)
	case "domain":
		domain := p.Domain
		if domain == "" {
			domain = "unknown"
		}
		base = fmt.Sprintf("Domain: %s (%s) [id: %s]", domain, p.Role, p.Id)
	default:
		base = fmt.Sprintf("%s (%s) [id: %s]", p.Type, p.Role, p.Id)
	}

	var extras []string
	if p.ExpirationTime != "" {
		extras = append(extras, "expires: "+p.ExpirationTime)
	}
	for _, detail := range p.PermissionDetails {
		if detail.Inherited && detail.InheritedFrom != "" {
			extras = append(extras, "inherited from: "+detail.InheritedFrom)
			break
		}
	}

	if len(extras) > 0 {
		return base + " | " + strings.Join(extras, ", ")
	}
	return base
}

// getBool extracts a bool param from the request, returning defaultVal if absent.
func getBool(request mcp.CallToolRequest, key string, defaultVal bool) bool {
	args := request.GetArguments()
	raw, ok := args[key]
	if !ok || raw == nil {
		return defaultVal
	}
	if v, ok := raw.(bool); ok {
		return v
	}
	return defaultVal
}

// detectSourceFormat detects the source MIME type from file name extension.
func detectSourceFormat(fileName, content string) string {
	idx := strings.LastIndex(fileName, ".")
	if idx >= 0 {
		ext := strings.ToLower(fileName[idx:])
		if mime, ok := googleDocsImportFormats[ext]; ok {
			return mime
		}
	}
	// Heuristic: if content looks like markdown, use markdown.
	if content != "" && (strings.HasPrefix(content, "#") || strings.Contains(content, "```") || strings.Contains(content, "**")) {
		return "text/markdown"
	}
	return "text/plain"
}
