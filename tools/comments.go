package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	drive "google.golang.org/api/drive/v3"
)

// RegisterCommentTools registers 4 comment management tools for a Google Workspace app.
// service is the MCP name prefix (docs, sheets, slides).
// appName is the human app type (document, spreadsheet, presentation).
// fileIDParam is the parameter name for the file ID (document_id, spreadsheet_id, presentation_id).
func RegisterCommentTools(s *mcpserver.MCPServer, getClient httpClientFunc, service, appName, fileIDParam string) {
	appTitle := titleCase(appName)

	// {service}_read_comments
	RegisterTool(s, mcp.NewTool(
		service+"_read_comments",
		mcp.WithDescription(fmt.Sprintf("List comments/threads on a Google %s. Required %s. Use for 'show review comments'. Not for file body — use Docs/Sheets/Slides read tools.", appTitle, fileIDParam)),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString(fileIDParam, mcp.Required(), mcp.Description(fmt.Sprintf("The ID of the Google %s.", appTitle))),
	), makeReadCommentsHandler(getClient, appName, fileIDParam))

	// {service}_create_comment
	RegisterTool(s, mcp.NewTool(
		service+"_create_comment",
		mcp.WithDescription(fmt.Sprintf("Add a new comment on a Google %s. Required %s + comment_content. Not a reply — use %s_reply_to_comment.", appTitle, fileIDParam, service)),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString(fileIDParam, mcp.Required(), mcp.Description(fmt.Sprintf("The ID of the Google %s.", appTitle))),
		mcp.WithString("comment_content", mcp.Required(), mcp.Description("The content of the comment.")),
	), makeCreateCommentHandler(getClient, appName, fileIDParam))

	// {service}_reply_to_comment
	RegisterTool(s, mcp.NewTool(
		service+"_reply_to_comment",
		mcp.WithDescription(fmt.Sprintf("Reply to a comment on a Google %s. Required %s, comment_id, reply_content. comment_id from %s_read_comments.", appTitle, fileIDParam, service)),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString(fileIDParam, mcp.Required(), mcp.Description(fmt.Sprintf("The ID of the Google %s.", appTitle))),
		mcp.WithString("comment_id", mcp.Required(), mcp.Description("The ID of the comment to reply to.")),
		mcp.WithString("reply_content", mcp.Required(), mcp.Description("The content of the reply.")),
	), makeReplyToCommentHandler(getClient, appName, fileIDParam))

	// {service}_resolve_comment
	RegisterTool(s, mcp.NewTool(
		service+"_resolve_comment",
		mcp.WithDescription(fmt.Sprintf("Resolve (close) a comment on a Google %s. Required %s + comment_id. Confirm when unclear.", appTitle, fileIDParam)),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString(fileIDParam, mcp.Required(), mcp.Description(fmt.Sprintf("The ID of the Google %s.", appTitle))),
		mcp.WithString("comment_id", mcp.Required(), mcp.Description("The ID of the comment to resolve.")),
	), makeResolveCommentHandler(getClient, appName, fileIDParam))
}

// titleCase returns the first letter capitalized, rest lower (simple single-word).
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// makeReadCommentsHandler creates a handler that reads all comments from a file.
func makeReadCommentsHandler(getClient httpClientFunc, appName, fileIDParam string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fileID, err := request.RequireString(fileIDParam)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.Comments.List(fileID).
			Fields("comments(id,content,author,createdTime,modifiedTime,resolved,replies(content,author,id,createdTime,modifiedTime))").
			Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing comments: %v", err)), nil
		}

		if len(resp.Comments) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No comments found in %s %s", appName, fileID)), nil
		}

		var out strings.Builder
		fmt.Fprintf(&out, "Found %d comments in %s %s:\n", len(resp.Comments), appName, fileID)

		for _, c := range resp.Comments {
			author := "Unknown"
			if c.Author != nil && c.Author.DisplayName != "" {
				author = c.Author.DisplayName
			}
			status := ""
			if c.Resolved {
				status = " [RESOLVED]"
			}

			fmt.Fprintf(&out, "\nComment ID: %s\n", c.Id)
			fmt.Fprintf(&out, "Author: %s\n", author)
			fmt.Fprintf(&out, "Created: %s%s\n", c.CreatedTime, status)
			fmt.Fprintf(&out, "Content: %s\n", c.Content)

			if len(c.Replies) > 0 {
				fmt.Fprintf(&out, "  Replies (%d):\n", len(c.Replies))
				for _, r := range c.Replies {
					replyAuthor := "Unknown"
					if r.Author != nil && r.Author.DisplayName != "" {
						replyAuthor = r.Author.DisplayName
					}
					fmt.Fprintf(&out, "    Reply ID: %s\n", r.Id)
					fmt.Fprintf(&out, "    Author: %s\n", replyAuthor)
					fmt.Fprintf(&out, "    Created: %s\n", r.CreatedTime)
					fmt.Fprintf(&out, "    Content: %s\n", r.Content)
				}
			}
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

// makeCreateCommentHandler creates a handler that creates a new comment on a file.
func makeCreateCommentHandler(getClient httpClientFunc, _, fileIDParam string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fileID, err := request.RequireString(fileIDParam)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		content, err := request.RequireString("comment_content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		comment, err := svc.Comments.Create(fileID, &drive.Comment{
			Content: content,
		}).Fields("id,content,author,createdTime,modifiedTime").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating comment: %v", err)), nil
		}

		author := "Unknown"
		if comment.Author != nil && comment.Author.DisplayName != "" {
			author = comment.Author.DisplayName
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Comment created successfully!\nComment ID: %s\nAuthor: %s\nCreated: %s\nContent: %s",
			comment.Id, author, comment.CreatedTime, content,
		)), nil
	}
}

// makeReplyToCommentHandler creates a handler that replies to a comment on a file.
func makeReplyToCommentHandler(getClient httpClientFunc, _, fileIDParam string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fileID, err := request.RequireString(fileIDParam)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		commentID, err := request.RequireString("comment_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		replyContent, err := request.RequireString("reply_content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		reply, err := svc.Replies.Create(fileID, commentID, &drive.Reply{
			Content: replyContent,
		}).Fields("id,content,author,createdTime,modifiedTime").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("replying to comment: %v", err)), nil
		}

		author := "Unknown"
		if reply.Author != nil && reply.Author.DisplayName != "" {
			author = reply.Author.DisplayName
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Reply posted successfully!\nReply ID: %s\nAuthor: %s\nCreated: %s\nContent: %s",
			reply.Id, author, reply.CreatedTime, replyContent,
		)), nil
	}
}

// makeResolveCommentHandler creates a handler that resolves a comment on a file.
func makeResolveCommentHandler(getClient httpClientFunc, _, fileIDParam string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		fileID, err := request.RequireString(fileIDParam)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		commentID, err := request.RequireString("comment_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		svc, err := newDriveService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		reply, err := svc.Replies.Create(fileID, commentID, &drive.Reply{
			Content: "This comment has been resolved.",
			Action:  "resolve",
		}).Fields("id,content,author,createdTime,modifiedTime").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolving comment: %v", err)), nil
		}

		author := "Unknown"
		if reply.Author != nil && reply.Author.DisplayName != "" {
			author = reply.Author.DisplayName
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Comment %s has been resolved successfully.\nResolve reply ID: %s\nAuthor: %s\nCreated: %s",
			commentID, reply.Id, author, reply.CreatedTime,
		)), nil
	}
}
