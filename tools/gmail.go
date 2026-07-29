package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	gmail "google.golang.org/api/gmail/v1"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// gmailMetadataHeaders are the headers to request in metadata-only fetches.
var gmailMetadataHeaders = []string{"Subject", "From", "To", "Cc", "Date", "Message-ID"}

// gmailBatchSize is the max messages/threads per batch request.
const gmailBatchSize = 25

// RegisterGmailTools registers all Gmail tools with the MCP server.
func RegisterGmailTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	// Read tools
	registerSearchGmailMessages(s, getClient)
	registerGetGmailMessageContent(s, getClient)
	registerGetGmailMessagesContentBatch(s, getClient)
	registerGetGmailAttachmentContent(s, getClient)
	registerGetGmailThreadContent(s, getClient)
	registerGetGmailThreadsContentBatch(s, getClient)
	registerListGmailLabels(s, getClient)

	// Write tools
	registerSendGmailMessage(s, getClient)
	registerDraftGmailMessage(s, getClient)
	registerManageGmailLabel(s, getClient)
	registerListGmailFilters(s, getClient)
	registerCreateGmailFilter(s, getClient)
	registerDeleteGmailFilter(s, getClient)
	registerModifyGmailMessageLabels(s, getClient)
	registerBatchModifyGmailMessageLabels(s, getClient)
}

// newGmailService creates a gmail.Service for the given user email.
func newGmailService(ctx context.Context, getClient httpClientFunc, email string) (*gmail.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := gmail.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Gmail service: %w", err)
	}
	return svc, nil
}

// --- gmail_search_messages ---

func registerSearchGmailMessages(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_search_messages",
		mcp.WithDescription("Search Gmail (from:, subject:, newer_than:, is:unread, …). Returns message/thread ids. Then gmail_get_message or gmail_get_messages_batch. Send → gmail_send_message."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Gmail search query.")),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithNumber("page_size", mcp.Description("Max messages. Default: 10.")),
		mcp.WithString("page_token", mcp.Description("Next page token from a prior result.")),
	)
	s.AddTool(tool, handleSearchGmailMessages(getClient))
}

func handleSearchGmailMessages(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return needArg("query", `gmail_search_messages(query="from:… newer_than:7d")`), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `gmail_search_messages(query="…")`), nil
		}
		pageSize := request.GetInt("page_size", 10)
		pageToken := request.GetString("page_token", "")

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		call := svc.Users.Messages.List("me").Q(query).MaxResults(int64(pageSize))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		messages := resp.Messages
		if messages == nil {
			messages = []*gmail.Message{}
		}

		return mcp.NewToolResultText(formatSearchResults(messages, query, resp.NextPageToken)), nil
	}
}

func formatSearchResults(messages []*gmail.Message, query, nextPageToken string) string {
	if len(messages) == 0 {
		return fmt.Sprintf("No messages found for query: '%s'", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d messages for query: '%s'\n\n", len(messages), query)

	for i, msg := range messages {
		fmt.Fprintf(&b, "%d. Message ID: %s\n", i+1, msg.Id)
		fmt.Fprintf(&b, "   Thread ID: %s\n", msg.ThreadId)
		fmt.Fprintf(&b, "   Web Link: https://mail.google.com/mail/u/0/#inbox/%s\n", msg.Id)
		if i < len(messages)-1 {
			b.WriteString("\n")
		}
	}

	if nextPageToken != "" {
		fmt.Fprintf(&b, "\n\nMore results available. Use page_token: '%s' to get the next page.", nextPageToken)
	}

	return b.String()
}

// --- gmail_get_message ---

func registerGetGmailMessageContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_get_message",
		mcp.WithDescription("Read one message by message_id (subject/from/body). After search. Many ids → gmail_get_messages_batch. Thread → gmail_get_thread."),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("Gmail message id from gmail_search_messages.")),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
	)
	s.AddTool(tool, handleGetGmailMessageContent(getClient))
}

func handleGetGmailMessageContent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		messageID, err := request.RequireString("message_id")
		if err != nil {
			return needArg("message_id", `gmail_search_messages(query="…") then gmail_get_message(message_id)`), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "gmail_get_message(message_id=…)"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch metadata headers.
		metaMsg, err := svc.Users.Messages.Get("me", messageID).Format("metadata").MetadataHeaders(gmailMetadataHeaders...).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}
		headers := extractHeaders(metaMsg.Payload, gmailMetadataHeaders)

		// Fetch full message for body.
		fullMsg, err := svc.Users.Messages.Get("me", messageID).Format("full").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		textBody, htmlBody := extractMessageBodies(fullMsg.Payload)
		bodyData := formatBodyContent(textBody, htmlBody)
		attachments := extractAttachments(fullMsg.Payload)

		return mcp.NewToolResultText(formatMessageContent(messageID, headers, bodyData, attachments)), nil
	}
}

func formatMessageContent(messageID string, headers map[string]string, bodyData string, attachments []attachmentMeta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Subject: %s\n", headerOrDefault(headers, "Subject", "(no subject)"))
	fmt.Fprintf(&b, "From:    %s\n", headerOrDefault(headers, "From", "(unknown sender)"))
	fmt.Fprintf(&b, "Date:    %s\n", headerOrDefault(headers, "Date", "(unknown date)"))

	if msgID := headers["Message-ID"]; msgID != "" {
		fmt.Fprintf(&b, "Message-ID: %s\n", msgID)
	}
	if to := headers["To"]; to != "" {
		fmt.Fprintf(&b, "To:      %s\n", to)
	}
	if cc := headers["Cc"]; cc != "" {
		fmt.Fprintf(&b, "Cc:      %s\n", cc)
	}

	if bodyData == "" {
		bodyData = "[No text/plain body found]"
	}
	fmt.Fprintf(&b, "\n--- BODY ---\n%s", bodyData)

	if len(attachments) > 0 {
		b.WriteString("\n\n--- ATTACHMENTS ---\n")
		for i, att := range attachments {
			sizeKB := float64(att.size) / 1024
			fmt.Fprintf(&b, "%d. %s (%s, %.1f KB)\n", i+1, att.filename, att.mimeType, sizeKB)
			fmt.Fprintf(&b, "   Attachment ID: %s\n", att.attachmentID)
			fmt.Fprintf(&b, "   Use gmail_get_attachment(message_id='%s', attachment_id='%s') to download\n", messageID, att.attachmentID)
		}
	}

	return b.String()
}

// --- gmail_get_messages_batch ---

func registerGetGmailMessagesContentBatch(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_get_messages_batch",
		mcp.WithDescription("Batch-read up to 25 messages by id (subject/from/body). Prefer over repeated gmail_get_message."),
		mcp.WithArray("message_ids",
			mcp.Required(),
			mcp.Description("Message ids from gmail_search_messages (max 25)."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
	)
	s.AddTool(tool, handleGetGmailMessagesContentBatch(getClient))
}

func handleGetGmailMessagesContentBatch(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		messageIDs, err := request.RequireStringSlice("message_ids")
		if err != nil {
			return needArg("message_ids", `gmail_get_messages_batch(message_ids=["id1","id2"])`), nil
		}
		if len(messageIDs) == 0 {
			return needArg("message_ids", `gmail_get_messages_batch(message_ids=["id1","id2"])`), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `gmail_get_messages_batch(message_ids=[…])`), nil
		}
		format := request.GetString("format", "full")

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var outputMessages []string

		// Process in chunks of gmailBatchSize.
		for chunkStart := 0; chunkStart < len(messageIDs); chunkStart += gmailBatchSize {
			chunkEnd := min(chunkStart+gmailBatchSize, len(messageIDs))
			chunk := messageIDs[chunkStart:chunkEnd]

			for _, mid := range chunk {
				var msg *gmail.Message
				var fetchErr error

				if format == "metadata" {
					msg, fetchErr = svc.Users.Messages.Get("me", mid).Format("metadata").MetadataHeaders(gmailMetadataHeaders...).Do()
				} else {
					msg, fetchErr = svc.Users.Messages.Get("me", mid).Format("full").Do()
				}

				if fetchErr != nil {
					outputMessages = append(outputMessages, fmt.Sprintf("Message %s: %v", mid, fetchErr))
					continue
				}

				outputMessages = append(outputMessages, formatBatchMessage(mid, msg, format))
			}
		}

		result := fmt.Sprintf("Retrieved %d messages:\n\n%s", len(messageIDs), strings.Join(outputMessages, "\n---\n\n"))
		return mcp.NewToolResultText(result), nil
	}
}

func formatBatchMessage(mid string, msg *gmail.Message, format string) string {
	headers := extractHeaders(msg.Payload, gmailMetadataHeaders)

	var b strings.Builder
	fmt.Fprintf(&b, "Message ID: %s\n", mid)
	fmt.Fprintf(&b, "Subject: %s\n", headerOrDefault(headers, "Subject", "(no subject)"))
	fmt.Fprintf(&b, "From: %s\n", headerOrDefault(headers, "From", "(unknown sender)"))
	fmt.Fprintf(&b, "Date: %s\n", headerOrDefault(headers, "Date", "(unknown date)"))

	if msgID := headers["Message-ID"]; msgID != "" {
		fmt.Fprintf(&b, "Message-ID: %s\n", msgID)
	}
	if to := headers["To"]; to != "" {
		fmt.Fprintf(&b, "To: %s\n", to)
	}
	if cc := headers["Cc"]; cc != "" {
		fmt.Fprintf(&b, "Cc: %s\n", cc)
	}

	fmt.Fprintf(&b, "Web Link: https://mail.google.com/mail/u/0/#inbox/%s\n", mid)

	if format == "full" {
		textBody, htmlBody := extractMessageBodies(msg.Payload)
		bodyData := formatBodyContent(textBody, htmlBody)
		if bodyData != "" {
			fmt.Fprintf(&b, "\n%s\n", bodyData)
		}
	}

	return b.String()
}

// --- gmail_get_attachment ---

func registerGetGmailAttachmentContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_get_attachment",
		mcp.WithDescription("Download one Gmail attachment by message_id + attachment_id. Use when the user needs a file from mail. Not for message body — use gmail_get_message."),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("The ID of the Gmail message containing the attachment.")),
		mcp.WithString("attachment_id", mcp.Required(), mcp.Description("The ID of the attachment to download.")),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
	)
	s.AddTool(tool, handleGetGmailAttachmentContent(getClient))
}

func handleGetGmailAttachmentContent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		messageID, err := request.RequireString("message_id")
		if err != nil {
			return mcp.NewToolResultError("message_id is required"), nil
		}
		attachmentID, err := request.RequireString("attachment_id")
		if err != nil {
			return mcp.NewToolResultError("attachment_id is required"), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		att, err := svc.Users.Messages.Attachments.Get("me", messageID, attachmentID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"Error: Failed to download attachment. The attachment ID may have changed.\n"+
					"Please fetch the message content again to get an updated attachment ID.\n\n"+
					"Error details: %v", err)), nil
		}

		sizeBytes := att.Size
		sizeKB := float64(sizeBytes) / 1024
		data := att.Data

		var b strings.Builder
		b.WriteString("Attachment downloaded successfully!\n")
		fmt.Fprintf(&b, "Message ID: %s\n", messageID)
		fmt.Fprintf(&b, "Size: %.1f KB (%d bytes)\n", sizeKB, sizeBytes)
		b.WriteString("\nBase64-encoded content (first 100 characters shown):\n")
		preview := data
		if len(preview) > 100 {
			preview = preview[:100]
		}
		fmt.Fprintf(&b, "%s...\n", preview)
		b.WriteString("\nNote: Attachment IDs are ephemeral. Always use IDs from the most recent message fetch.")

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- gmail_get_thread ---

func registerGetGmailThreadContent(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_get_thread",
		mcp.WithDescription("Read a full Gmail thread by thread_id (all messages). Use for 'show the whole conversation'. Prefer gmail_get_message for a single message."),
		mcp.WithString("thread_id", mcp.Required(), mcp.Description("The unique ID of the Gmail thread to retrieve.")),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
	)
	s.AddTool(tool, handleGetGmailThreadContent(getClient))
}

func handleGetGmailThreadContent(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		threadID, err := request.RequireString("thread_id")
		if err != nil {
			return mcp.NewToolResultError("thread_id is required"), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		thread, err := svc.Users.Threads.Get("me", threadID).Format("full").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		return mcp.NewToolResultText(formatThreadContent(thread, threadID)), nil
	}
}

func formatThreadContent(thread *gmail.Thread, threadID string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Thread ID: %s\n", threadID)
	fmt.Fprintf(&b, "Messages in thread: %d\n\n", len(thread.Messages))

	for i, msg := range thread.Messages {
		headers := extractHeaders(msg.Payload, gmailMetadataHeaders)
		fmt.Fprintf(&b, "--- Message %d of %d ---\n", i+1, len(thread.Messages))
		fmt.Fprintf(&b, "Message ID: %s\n", msg.Id)
		fmt.Fprintf(&b, "Subject: %s\n", headerOrDefault(headers, "Subject", "(no subject)"))
		fmt.Fprintf(&b, "From: %s\n", headerOrDefault(headers, "From", "(unknown sender)"))
		fmt.Fprintf(&b, "Date: %s\n", headerOrDefault(headers, "Date", "(unknown date)"))
		if to := headers["To"]; to != "" {
			fmt.Fprintf(&b, "To: %s\n", to)
		}
		if cc := headers["Cc"]; cc != "" {
			fmt.Fprintf(&b, "Cc: %s\n", cc)
		}

		textBody, htmlBody := extractMessageBodies(msg.Payload)
		bodyData := formatBodyContent(textBody, htmlBody)
		if bodyData != "" {
			fmt.Fprintf(&b, "\n%s\n", bodyData)
		} else {
			b.WriteString("\n[No body content]\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// --- gmail_get_threads_batch ---

func registerGetGmailThreadsContentBatch(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_get_threads_batch",
		mcp.WithDescription("Batch-read up to 25 Gmail threads by ID. Use when summarizing several conversations. Prefer gmail_get_thread for one thread."),
		mcp.WithArray("thread_ids",
			mcp.Required(),
			mcp.Description("A list of Gmail thread IDs to retrieve. The function will automatically batch requests in chunks of 25."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
	)
	s.AddTool(tool, handleGetGmailThreadsContentBatch(getClient))
}

func handleGetGmailThreadsContentBatch(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		threadIDs, err := request.RequireStringSlice("thread_ids")
		if err != nil {
			return mcp.NewToolResultError("thread_ids is required"), nil
		}
		if len(threadIDs) == 0 {
			return mcp.NewToolResultError("No thread IDs provided"), nil
		}
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var outputThreads []string

		for chunkStart := 0; chunkStart < len(threadIDs); chunkStart += gmailBatchSize {
			chunkEnd := min(chunkStart+gmailBatchSize, len(threadIDs))
			chunk := threadIDs[chunkStart:chunkEnd]

			for _, tid := range chunk {
				thread, fetchErr := svc.Users.Threads.Get("me", tid).Format("full").Do()
				if fetchErr != nil {
					outputThreads = append(outputThreads, fmt.Sprintf("Thread %s: %v", tid, fetchErr))
					continue
				}
				outputThreads = append(outputThreads, formatThreadContent(thread, tid))
			}
		}

		result := fmt.Sprintf("Retrieved %d threads:\n\n%s", len(threadIDs), strings.Join(outputThreads, "\n---\n\n"))
		return mcp.NewToolResultText(result), nil
	}
}

// --- gmail_list_labels ---

func registerListGmailLabels(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_list_labels",
		mcp.WithDescription("List Gmail labels (ids + names). Use before apply/remove labels or to discover label ids. Not for reading mail — use gmail_search_messages."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address. Required.")),
	)
	s.AddTool(tool, handleListGmailLabels(getClient))
}

func handleListGmailLabels(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.Users.Labels.List("me").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		labels := resp.Labels
		if len(labels) == 0 {
			return mcp.NewToolResultText("No labels found."), nil
		}

		var systemLabels, userLabels []*gmail.Label
		for _, l := range labels {
			if l.Type == "system" {
				systemLabels = append(systemLabels, l)
			} else {
				userLabels = append(userLabels, l)
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d labels:\n", len(labels))

		if len(systemLabels) > 0 {
			b.WriteString("\nSYSTEM LABELS:\n")
			for _, l := range systemLabels {
				fmt.Fprintf(&b, "  - %s (ID: %s)\n", l.Name, l.Id)
			}
		}

		if len(userLabels) > 0 {
			b.WriteString("\nUSER LABELS:\n")
			for _, l := range userLabels {
				fmt.Fprintf(&b, "  - %s (ID: %s)\n", l.Name, l.Id)
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- gmail_send_message ---

func registerSendGmailMessage(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_send_message",
		mcp.WithDescription("Send email now. Confirm with user unless they explicitly said send. Draft → gmail_draft_message. Read → gmail_search_messages / gmail_get_message."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("to", mcp.Required(), mcp.Description("Recipient email.")),
		mcp.WithString("subject", mcp.Required(), mcp.Description("Subject.")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Body (plain text or HTML).")),
		mcp.WithString("cc", mcp.Description("CC email.")),
		mcp.WithString("bcc", mcp.Description("BCC email.")),
		mcp.WithString("thread_id", mcp.Description("Thread id to reply in.")),
		mcp.WithArray("attachments",
			mcp.Description("Optional attachments: {\"path\":\"/file.pdf\"} or {\"filename\",\"content\"(base64),\"mime_type\"}."),
			mcp.Items(map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}),
		),
	)
	s.AddTool(tool, handleSendGmailMessage(getClient))
}

func handleSendGmailMessage(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "gmail_send_message(to, subject, body)"), nil
		}
		to, err := request.RequireString("to")
		if err != nil {
			return needArg("to", "gmail_send_message(to, subject, body)"), nil
		}
		subject, err := request.RequireString("subject")
		if err != nil {
			return needArg("subject", "gmail_send_message(to, subject, body)"), nil
		}
		body, err := request.RequireString("body")
		if err != nil {
			return needArg("body", "gmail_send_message(to, subject, body)"), nil
		}

		bodyFormat := request.GetString("body_format", "plain")
		ccAddr := request.GetString("cc", "")
		bccAddr := request.GetString("bcc", "")
		fromName := request.GetString("from_name", "")
		fromEmail := request.GetString("from_email", "")
		threadID := request.GetString("thread_id", "")
		inReplyTo := request.GetString("in_reply_to", "")
		references := request.GetString("references", "")

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sender := email
		if fromEmail != "" {
			sender = fromEmail
		}

		// Auto-prefix "Re: " for replies.
		if inReplyTo != "" && !strings.HasPrefix(strings.ToLower(subject), "re:") {
			subject = "Re: " + subject
		}

		attachments := getAttachments(request)

		raw := buildRawMessage(sender, fromName, to, ccAddr, bccAddr, subject, body, bodyFormat, inReplyTo, references, attachments)

		msg := &gmail.Message{Raw: raw}
		if threadID != "" {
			msg.ThreadId = threadID
		}

		sent, err := svc.Users.Messages.Send("me", msg).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		return mcp.NewToolResultText("Email sent successfully!\nMessage ID: " + sent.Id), nil
	}
}

// --- gmail_draft_message ---

func registerDraftGmailMessage(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_draft_message",
		mcp.WithDescription("Create a Gmail DRAFT (not sent). Use for 'draft a reply', prepare mail for later. To send immediately use gmail_send_message after confirmation."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("subject", mcp.Required(), mcp.Description("Email subject.")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Email body content (plain text or HTML).")),
		mcp.WithString("to", mcp.Description("Recipient email address.")),
		mcp.WithString("body_format", mcp.Description("Email body format."), mcp.Enum("plain", "html")),
		mcp.WithString("cc", mcp.Description("CC email address.")),
		mcp.WithString("bcc", mcp.Description("BCC email address.")),
		mcp.WithString("from_name", mcp.Description("Sender display name for \"Name <email>\" formatting.")),
		mcp.WithString("from_email", mcp.Description("\"Send As\" alias email (must be configured in Gmail settings).")),
		mcp.WithString("thread_id", mcp.Description("Gmail thread ID to reply within.")),
		mcp.WithString("in_reply_to", mcp.Description("Message-ID of message being replied to.")),
		mcp.WithString("references", mcp.Description("Chain of Message-IDs for proper threading.")),
		mcp.WithArray("attachments",
			mcp.Description("Optional list of attachments. Each can have: 'path' (file path, auto-encodes), OR 'content' (standard base64, not urlsafe) + 'filename'. Optional 'mime_type' (auto-detected from path if not provided)."),
			mcp.Items(map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}),
		),
	)
	s.AddTool(tool, handleDraftGmailMessage(getClient))
}

func handleDraftGmailMessage(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		subject, err := request.RequireString("subject")
		if err != nil {
			return mcp.NewToolResultError("subject is required"), nil
		}
		body, err := request.RequireString("body")
		if err != nil {
			return mcp.NewToolResultError("body is required"), nil
		}

		toAddr := request.GetString("to", "")
		bodyFormat := request.GetString("body_format", "plain")
		ccAddr := request.GetString("cc", "")
		bccAddr := request.GetString("bcc", "")
		fromName := request.GetString("from_name", "")
		fromEmail := request.GetString("from_email", "")
		threadID := request.GetString("thread_id", "")
		inReplyTo := request.GetString("in_reply_to", "")
		references := request.GetString("references", "")

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sender := email
		if fromEmail != "" {
			sender = fromEmail
		}

		if inReplyTo != "" && !strings.HasPrefix(strings.ToLower(subject), "re:") {
			subject = "Re: " + subject
		}

		attachments := getAttachments(request)

		raw := buildRawMessage(sender, fromName, toAddr, ccAddr, bccAddr, subject, body, bodyFormat, inReplyTo, references, attachments)

		innerMsg := &gmail.Message{Raw: raw}
		if threadID != "" {
			innerMsg.ThreadId = threadID
		}

		draft := &gmail.Draft{Message: innerMsg}

		created, err := svc.Users.Drafts.Create("me", draft).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		return mcp.NewToolResultText("Draft created successfully!\nDraft ID: " + created.Id), nil
	}
}

// --- gmail_manage_label ---

func registerManageGmailLabel(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_manage_label",
		mcp.WithDescription("Create/update/delete Gmail labels (label definitions). Use for 'make a label called…'. To apply labels to messages use gmail_modify_message_labels."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform."), mcp.Enum("create", "update", "delete")),
		mcp.WithString("name", mcp.Description("Label name. Required for create, optional for update.")),
		mcp.WithString("label_id", mcp.Description("Label ID. Required for update and delete.")),
		mcp.WithString("label_list_visibility", mcp.Description("Show in label list."), mcp.Enum("labelShow", "labelHide")),
		mcp.WithString("message_list_visibility", mcp.Description("Show in message list."), mcp.Enum("show", "hide")),
	)
	s.AddTool(tool, handleManageGmailLabel(getClient))
}

func handleManageGmailLabel(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		action, err := request.RequireString("action")
		if err != nil {
			return mcp.NewToolResultError("action is required"), nil
		}

		name := request.GetString("name", "")
		labelID := request.GetString("label_id", "")
		labelListVis := request.GetString("label_list_visibility", "labelShow")
		msgListVis := request.GetString("message_list_visibility", "show")

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch action {
		case "create":
			if name == "" {
				return mcp.NewToolResultError("name is required for create action"), nil
			}
			label := &gmail.Label{
				Name:                  name,
				LabelListVisibility:   labelListVis,
				MessageListVisibility: msgListVis,
			}
			created, err := svc.Users.Labels.Create("me", label).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Label created successfully!\nName: %s\nID: %s", created.Name, created.Id)), nil

		case "update":
			if labelID == "" {
				return mcp.NewToolResultError("label_id is required for update action"), nil
			}
			current, err := svc.Users.Labels.Get("me", labelID).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Gmail API error fetching label: %v", err)), nil
			}
			updateName := current.Name
			if name != "" {
				updateName = name
			}
			label := &gmail.Label{
				Name:                  updateName,
				LabelListVisibility:   labelListVis,
				MessageListVisibility: msgListVis,
			}
			updated, err := svc.Users.Labels.Update("me", labelID, label).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Label updated successfully!\nName: %s\nID: %s", updated.Name, updated.Id)), nil

		case "delete":
			if labelID == "" {
				return mcp.NewToolResultError("label_id is required for delete action"), nil
			}
			current, err := svc.Users.Labels.Get("me", labelID).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Gmail API error fetching label: %v", err)), nil
			}
			err = svc.Users.Labels.Delete("me", labelID).Do()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Label deleted successfully!\nName: %s\nID: %s", current.Name, current.Id)), nil

		default:
			return mcp.NewToolResultError("unknown action: " + action), nil
		}
	}
}

// --- gmail_list_filters ---

func registerListGmailFilters(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_list_filters",
		mcp.WithDescription("List configured Gmail filters. Use before create/delete filter or to audit automation rules."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
	)
	s.AddTool(tool, handleListGmailFilters(getClient))
}

func handleListGmailFilters(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := svc.Users.Settings.Filters.List("me").Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		filters := resp.Filter
		if len(filters) == 0 {
			return mcp.NewToolResultText("No filters found."), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d filters:\n\n", len(filters))
		for i, f := range filters {
			fmt.Fprintf(&b, "%d. Filter ID: %s\n", i+1, f.Id)
			if f.Criteria != nil {
				b.WriteString(formatGmailFilterCriteria(f.Criteria))
			}
			if f.Action != nil {
				b.WriteString("   Actions:\n")
				if f.Action.Forward != "" {
					fmt.Fprintf(&b, "     Forward to: %s\n", f.Action.Forward)
				}
				if len(f.Action.AddLabelIds) > 0 {
					fmt.Fprintf(&b, "     Add labels: %s\n", strings.Join(f.Action.AddLabelIds, ", "))
				}
				if len(f.Action.RemoveLabelIds) > 0 {
					fmt.Fprintf(&b, "     Remove labels: %s\n", strings.Join(f.Action.RemoveLabelIds, ", "))
				}
			}
			if i < len(filters)-1 {
				b.WriteString("\n")
			}
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- gmail_create_filter ---

func registerCreateGmailFilter(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_create_filter",
		mcp.WithDescription("Create a Gmail filter (criteria + actions). Use for 'auto-archive mail from…'. Confirm before creating noisy rules."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithObject("criteria", mcp.Required(), mcp.Description("Filter criteria object. Supports: from, to, subject, query, negatedQuery, hasAttachment, excludeChats, size, sizeComparison.")),
		mcp.WithObject("action", mcp.Required(), mcp.Description("Filter action object. Supports: forward, addLabelIds, removeLabelIds.")),
	)
	s.AddTool(tool, handleCreateGmailFilter(getClient))
}

func handleCreateGmailFilter(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}

		args := request.GetArguments()
		criteriaRaw, ok := args["criteria"].(map[string]any)
		if !ok {
			return mcp.NewToolResultError("criteria is required and must be an object"), nil
		}
		actionRaw, ok := args["action"].(map[string]any)
		if !ok {
			return mcp.NewToolResultError("action is required and must be an object"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		criteria := &gmail.FilterCriteria{}
		if v, ok := criteriaRaw["from"].(string); ok {
			criteria.From = v
		}
		if v, ok := criteriaRaw["to"].(string); ok {
			criteria.To = v
		}
		if v, ok := criteriaRaw["subject"].(string); ok {
			criteria.Subject = v
		}
		if v, ok := criteriaRaw["query"].(string); ok {
			criteria.Query = v
		}
		if v, ok := criteriaRaw["negatedQuery"].(string); ok {
			criteria.NegatedQuery = v
		}
		if v, ok := criteriaRaw["hasAttachment"].(bool); ok {
			criteria.HasAttachment = v
		}
		if v, ok := criteriaRaw["excludeChats"].(bool); ok {
			criteria.ExcludeChats = v
		}
		if v, ok := criteriaRaw["size"].(float64); ok {
			criteria.Size = int64(v)
		}
		if v, ok := criteriaRaw["sizeComparison"].(string); ok {
			criteria.SizeComparison = v
		}

		action := &gmail.FilterAction{}
		if v, ok := actionRaw["forward"].(string); ok {
			action.Forward = v
		}
		if v, ok := actionRaw["addLabelIds"].([]any); ok {
			for _, id := range v {
				if s, ok := id.(string); ok {
					action.AddLabelIds = append(action.AddLabelIds, s)
				}
			}
		}
		if v, ok := actionRaw["removeLabelIds"].([]any); ok {
			for _, id := range v {
				if s, ok := id.(string); ok {
					action.RemoveLabelIds = append(action.RemoveLabelIds, s)
				}
			}
		}

		filter := &gmail.Filter{
			Criteria: criteria,
			Action:   action,
		}

		created, err := svc.Users.Settings.Filters.Create("me", filter).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		return mcp.NewToolResultText("Filter created successfully!\nFilter ID: " + created.Id), nil
	}
}

// --- gmail_delete_filter ---

func registerDeleteGmailFilter(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_delete_filter",
		mcp.WithDescription("Delete a Gmail filter by id (from gmail_list_filters). Destructive — confirm when unclear."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("filter_id", mcp.Required(), mcp.Description("ID of the filter to delete.")),
	)
	s.AddTool(tool, handleDeleteGmailFilter(getClient))
}

func handleDeleteGmailFilter(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		filterID, err := request.RequireString("filter_id")
		if err != nil {
			return mcp.NewToolResultError("filter_id is required"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch filter details before deleting.
		filter, err := svc.Users.Settings.Filters.Get("me", filterID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error fetching filter: %v", err)), nil
		}

		err = svc.Users.Settings.Filters.Delete("me", filterID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		criteriaStr := "(none)"
		if filter.Criteria != nil {
			criteriaStr = describeGmailFilterCriteria(filter.Criteria)
		}

		actionStr := "(none)"
		if filter.Action != nil {
			var parts []string
			if filter.Action.Forward != "" {
				parts = append(parts, "forward:"+filter.Action.Forward)
			}
			if len(filter.Action.AddLabelIds) > 0 {
				parts = append(parts, "addLabels:"+strings.Join(filter.Action.AddLabelIds, ","))
			}
			if len(filter.Action.RemoveLabelIds) > 0 {
				parts = append(parts, "removeLabels:"+strings.Join(filter.Action.RemoveLabelIds, ","))
			}
			if len(parts) > 0 {
				actionStr = strings.Join(parts, ", ")
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("Filter deleted successfully!\nFilter ID: %s\nCriteria: %s\nAction: %s", filterID, criteriaStr, actionStr)), nil
	}
}

func formatGmailFilterCriteria(criteria *gmail.FilterCriteria) string {
	var b strings.Builder
	b.WriteString("   Criteria:\n")
	for _, field := range []struct {
		name, value string
	}{
		{"From", criteria.From},
		{"To", criteria.To},
		{"Subject", criteria.Subject},
		{"Query", criteria.Query},
		{"Negated Query", criteria.NegatedQuery},
	} {
		if field.value != "" {
			fmt.Fprintf(&b, "     %s: %s\n", field.name, field.value)
		}
	}
	if criteria.HasAttachment {
		b.WriteString("     Has Attachment: true\n")
	}
	if criteria.ExcludeChats {
		b.WriteString("     Exclude Chats: true\n")
	}
	if criteria.Size > 0 {
		fmt.Fprintf(&b, "     Size: %d (%s)\n", criteria.Size, criteria.SizeComparison)
	}
	return b.String()
}

func describeGmailFilterCriteria(criteria *gmail.FilterCriteria) string {
	var parts []string
	for _, field := range []struct {
		name, value string
	}{
		{"from", criteria.From},
		{"to", criteria.To},
		{"subject", criteria.Subject},
		{"query", criteria.Query},
	} {
		if field.value != "" {
			parts = append(parts, field.name+":"+field.value)
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ", ")
}

// --- gmail_modify_message_labels ---

func registerModifyGmailMessageLabels(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_modify_message_labels",
		mcp.WithDescription("Add/remove labels on one message. Archive: remove INBOX. Trash: add TRASH. Use after search/get for message_id. For many messages use gmail_batch_modify_message_labels."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("ID of the message to modify.")),
		mcp.WithArray("add_label_ids",
			mcp.Description("Label IDs to add."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("remove_label_ids",
			mcp.Description("Label IDs to remove."),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
	s.AddTool(tool, handleModifyGmailMessageLabels(getClient))
}

func handleModifyGmailMessageLabels(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		messageID, err := request.RequireString("message_id")
		if err != nil {
			return mcp.NewToolResultError("message_id is required"), nil
		}

		addLabels := getStringSlice(request, "add_label_ids")
		removeLabels := getStringSlice(request, "remove_label_ids")

		if len(addLabels) == 0 && len(removeLabels) == 0 {
			return mcp.NewToolResultError("at least one of add_label_ids or remove_label_ids must be provided"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		modReq := &gmail.ModifyMessageRequest{}
		if len(addLabels) > 0 {
			modReq.AddLabelIds = addLabels
		}
		if len(removeLabels) > 0 {
			modReq.RemoveLabelIds = removeLabels
		}

		_, err = svc.Users.Messages.Modify("me", messageID, modReq).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Labels modified for message %s\n", messageID)
		if len(addLabels) > 0 {
			fmt.Fprintf(&b, "Added: %s\n", strings.Join(addLabels, ", "))
		}
		if len(removeLabels) > 0 {
			fmt.Fprintf(&b, "Removed: %s\n", strings.Join(removeLabels, ", "))
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- gmail_batch_modify_message_labels ---

func registerBatchModifyGmailMessageLabels(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("gmail_batch_modify_message_labels",
		mcp.WithDescription("Batch add/remove labels on many message IDs. Use for bulk archive/label. Prefer gmail_modify_message_labels for one message."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithArray("message_ids",
			mcp.Required(),
			mcp.Description("List of message IDs to modify."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("add_label_ids",
			mcp.Description("Label IDs to add to messages."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("remove_label_ids",
			mcp.Description("Label IDs to remove from messages."),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
	s.AddTool(tool, handleBatchModifyGmailMessageLabels(getClient))
}

func handleBatchModifyGmailMessageLabels(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		messageIDs, err := request.RequireStringSlice("message_ids")
		if err != nil {
			return mcp.NewToolResultError("message_ids is required"), nil
		}
		if len(messageIDs) == 0 {
			return mcp.NewToolResultError("message_ids must not be empty"), nil
		}

		addLabels := getStringSlice(request, "add_label_ids")
		removeLabels := getStringSlice(request, "remove_label_ids")

		if len(addLabels) == 0 && len(removeLabels) == 0 {
			return mcp.NewToolResultError("at least one of add_label_ids or remove_label_ids must be provided"), nil
		}

		svc, err := newGmailService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		batchReq := &gmail.BatchModifyMessagesRequest{
			Ids: messageIDs,
		}
		if len(addLabels) > 0 {
			batchReq.AddLabelIds = addLabels
		}
		if len(removeLabels) > 0 {
			batchReq.RemoveLabelIds = removeLabels
		}

		err = svc.Users.Messages.BatchModify("me", batchReq).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Gmail API error: %v", err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Labels modified for %d messages\n", len(messageIDs))
		if len(addLabels) > 0 {
			fmt.Fprintf(&b, "Added: %s\n", strings.Join(addLabels, ", "))
		}
		if len(removeLabels) > 0 {
			fmt.Fprintf(&b, "Removed: %s\n", strings.Join(removeLabels, ", "))
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- Gmail write helper functions ---

// emailAttachment represents a parsed attachment for message building.
type emailAttachment struct {
	filename string
	mimeType string
	data     []byte
}

// getAttachments extracts attachment data from request params.
func getAttachments(request mcp.CallToolRequest) []emailAttachment {
	args := request.GetArguments()
	raw, ok := args["attachments"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	var result []emailAttachment
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		att := emailAttachment{}
		mimeType, _ := m["mime_type"].(string)

		if path, ok := m["path"].(string); ok && path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			att.data = data
			att.filename = filepath.Base(path)
			if mimeType == "" {
				mimeType = mime.TypeByExtension(filepath.Ext(path))
			}
		} else if content, ok := m["content"].(string); ok && content != "" {
			data, err := base64.StdEncoding.DecodeString(content)
			if err != nil {
				continue
			}
			att.data = data
			att.filename, _ = m["filename"].(string)
		}

		if att.filename == "" {
			att.filename = "attachment"
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		att.mimeType = mimeType

		if len(att.data) > 0 {
			result = append(result, att)
		}
	}
	return result
}

// buildRawMessage constructs a base64url-encoded RFC 2822 message with optional attachments.
func buildRawMessage(from, fromName, to, cc, bcc, subject, body, bodyFormat, inReplyTo, references string, attachments []emailAttachment) string {
	var msg strings.Builder

	// Common headers.
	if fromName != "" {
		fmt.Fprintf(&msg, "From: %s\r\n", formatAddress(fromName, from))
	} else {
		fmt.Fprintf(&msg, "From: %s\r\n", from)
	}

	if to != "" {
		fmt.Fprintf(&msg, "To: %s\r\n", to)
	}
	if cc != "" {
		fmt.Fprintf(&msg, "Cc: %s\r\n", cc)
	}
	if bcc != "" {
		fmt.Fprintf(&msg, "Bcc: %s\r\n", bcc)
	}

	fmt.Fprintf(&msg, "Subject: %s\r\n", encodeSubject(subject))

	if inReplyTo != "" {
		fmt.Fprintf(&msg, "In-Reply-To: %s\r\n", inReplyTo)
	}
	if references != "" {
		fmt.Fprintf(&msg, "References: %s\r\n", references)
	}

	msg.WriteString("MIME-Version: 1.0\r\n")

	contentType := "text/plain"
	if bodyFormat == "html" {
		contentType = "text/html"
	}

	if len(attachments) == 0 {
		// Simple message without attachments.
		fmt.Fprintf(&msg, "Content-Type: %s; charset=\"UTF-8\"\r\n", contentType)
		msg.WriteString("\r\n")
		msg.WriteString(body)
	} else {
		// Multipart message with attachments.
		boundary := "boundary_mcp_go_attachment"
		fmt.Fprintf(&msg, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
		msg.WriteString("\r\n")

		// Body part.
		fmt.Fprintf(&msg, "--%s\r\n", boundary)
		fmt.Fprintf(&msg, "Content-Type: %s; charset=\"UTF-8\"\r\n", contentType)
		msg.WriteString("\r\n")
		msg.WriteString(body)
		msg.WriteString("\r\n")

		// Attachment parts.
		for _, att := range attachments {
			fmt.Fprintf(&msg, "--%s\r\n", boundary)
			fmt.Fprintf(&msg, "Content-Type: %s; name=\"%s\"\r\n", att.mimeType, att.filename)
			msg.WriteString("Content-Transfer-Encoding: base64\r\n")
			fmt.Fprintf(&msg, "Content-Disposition: attachment; filename=\"%s\"\r\n", att.filename)
			msg.WriteString("\r\n")
			msg.WriteString(base64.StdEncoding.EncodeToString(att.data))
			msg.WriteString("\r\n")
		}

		fmt.Fprintf(&msg, "--%s--\r\n", boundary)
	}

	return base64.URLEncoding.EncodeToString([]byte(msg.String()))
}

// formatAddress formats "Name <email>" using RFC 2047 encoding for the name.
func formatAddress(name, email string) string {
	addr := mail.Address{Name: name, Address: email}
	return addr.String()
}

// encodeSubject encodes a subject line using RFC 2047 if needed.
func encodeSubject(subject string) string {
	for _, c := range subject {
		if c > 127 {
			return mime.QEncoding.Encode("utf-8", subject)
		}
	}
	return subject
}

// getStringSlice extracts a []string from request params, returning nil if absent/empty.
func getStringSlice(request mcp.CallToolRequest, key string) []string {
	args := request.GetArguments()
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// --- helper functions ---

// extractHeaders extracts the specified headers from a MessagePart payload.
func extractHeaders(payload *gmail.MessagePart, headerNames []string) map[string]string {
	result := make(map[string]string, len(headerNames))
	if payload == nil {
		return result
	}
	want := make(map[string]bool, len(headerNames))
	for _, h := range headerNames {
		want[h] = true
	}
	for _, h := range payload.Headers {
		if want[h.Name] {
			result[h.Name] = h.Value
		}
	}
	return result
}

// headerOrDefault returns the header value or a default.
func headerOrDefault(headers map[string]string, key, def string) string {
	if v, ok := headers[key]; ok && v != "" {
		return v
	}
	return def
}

// attachmentMeta holds metadata about an attachment.
type attachmentMeta struct {
	filename     string
	mimeType     string
	size         int64
	attachmentID string
}

// extractAttachments extracts attachment metadata from a message payload.
func extractAttachments(payload *gmail.MessagePart) []attachmentMeta {
	if payload == nil {
		return nil
	}
	var result []attachmentMeta
	var walk func(part *gmail.MessagePart)
	walk = func(part *gmail.MessagePart) {
		if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
			result = append(result, attachmentMeta{
				filename:     part.Filename,
				mimeType:     part.MimeType,
				size:         part.Body.Size,
				attachmentID: part.Body.AttachmentId,
			})
		}
		for _, child := range part.Parts {
			walk(child)
		}
	}
	walk(payload)
	return result
}

// extractMessageBodies extracts text/plain and text/html bodies from a message payload.
func extractMessageBodies(payload *gmail.MessagePart) (textBody, htmlBody string) {
	if payload == nil {
		return "", ""
	}

	var walk func(part *gmail.MessagePart)
	walk = func(part *gmail.MessagePart) {
		if part.MimeType == "text/plain" && textBody == "" {
			if part.Body != nil && part.Body.Data != "" {
				textBody = decodeBase64URL(part.Body.Data)
			}
		} else if part.MimeType == "text/html" && htmlBody == "" {
			if part.Body != nil && part.Body.Data != "" {
				htmlBody = decodeBase64URL(part.Body.Data)
			}
		}
		for _, child := range part.Parts {
			walk(child)
		}
	}
	walk(payload)
	return textBody, htmlBody
}

// formatBodyContent returns the text body, falling back to HTML body if empty.
func formatBodyContent(textBody, htmlBody string) string {
	if textBody != "" {
		return textBody
	}
	if htmlBody != "" {
		return "[HTML body - plain text not available]\n" + htmlBody
	}
	return ""
}

// decodeBase64URL decodes a base64url-encoded string (Gmail API format).
func decodeBase64URL(s string) string {
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		// Try with padding stripped (Gmail sometimes omits padding).
		data, err = base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return s // Return raw if decode fails.
		}
	}
	return string(data)
}
