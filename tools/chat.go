package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	chat "google.golang.org/api/chat/v1"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// RegisterChatTools registers all Chat tools with the MCP server.
func RegisterChatTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	registerListSpaces(s, getClient)
	registerGetMessages(s, getClient)
	registerSendMessage(s, getClient)
	registerSearchMessages(s, getClient)
}

// newChatService creates a chat.Service for the given user email.
func newChatService(ctx context.Context, getClient httpClientFunc, email string) (*chat.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := chat.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Chat service: %w", err)
	}
	return svc, nil
}

// normalizeChatSpaceID ensures space_id is a Chat resource name (spaces/…).
// Bare ids like "AAAA" become "spaces/AAAA". Display names are left unchanged
// so the API error path can teach chat_list_spaces.
func normalizeChatSpaceID(spaceID string) string {
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return ""
	}
	if strings.HasPrefix(spaceID, "spaces/") {
		return spaceID
	}
	// Already a nested resource (e.g. spaces/…/messages/…) — leave alone.
	if strings.Contains(spaceID, "/") {
		return spaceID
	}
	return "spaces/" + spaceID
}

// --- chat_list_spaces ---

func registerListSpaces(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("chat_list_spaces",
		mcp.WithDescription("List Chat spaces (space_id like spaces/…, DMs, rooms). Use before chat_list_messages or chat_send_message. Message search → chat_search_messages."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
	)
	s.AddTool(tool, handleListSpaces(getClient))
}

func handleListSpaces(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", "chat_list_spaces()"), nil
		}

		// Optional paging/filter still accepted if a client sends them (not in lean schema).
		pageSize := request.GetInt("page_size", 100)
		spaceType := request.GetString("space_type", "all")

		svc, err := newChatService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		call := svc.Spaces.List().PageSize(int64(pageSize))

		// Build filter based on space_type
		switch spaceType {
		case "room":
			call = call.Filter("spaceType = SPACE")
		case "dm":
			call = call.Filter("spaceType = DIRECT_MESSAGE")
		}

		resp, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing spaces: %v", err)), nil
		}

		spaces := resp.Spaces
		if len(spaces) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No Chat spaces found for type '%s'.", spaceType)), nil
		}

		var out strings.Builder
		fmt.Fprintf(&out, "Found %d Chat spaces (type: %s):", len(spaces), spaceType)
		for _, space := range spaces {
			name := space.DisplayName
			if name == "" {
				name = "Unnamed Space"
			}
			spaceTypeActual := space.SpaceType
			if spaceTypeActual == "" {
				spaceTypeActual = "UNKNOWN"
			}
			fmt.Fprintf(&out, "\n- %s (ID: %s, Type: %s)", name, space.Name, spaceTypeActual)
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

// --- chat_list_messages ---

func registerGetMessages(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("chat_list_messages",
		mcp.WithDescription("Read messages in space_id (paginated). Required: space_id from chat_list_spaces (spaces/…, not display name). Send → chat_send_message."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("space_id", mcp.Required(), mcp.Description("Chat space resource name from chat_list_spaces (spaces/…). Not a display name.")),
	)
	s.AddTool(tool, handleGetMessages(getClient))
}

func handleGetMessages(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `chat_list_messages(space_id="spaces/…")`), nil
		}
		spaceID, err := request.RequireString("space_id")
		if err != nil || strings.TrimSpace(spaceID) == "" {
			return needArg("space_id", `chat_list_spaces() then chat_list_messages(space_id="spaces/…")`), nil
		}
		spaceID = normalizeChatSpaceID(spaceID)

		// Optional paging/order still accepted if a client sends them (not in lean schema).
		pageSize := request.GetInt("page_size", 50)
		orderBy := request.GetString("order_by", "createTime desc")

		svc, err := newChatService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Get space info first
		spaceInfo, err := svc.Spaces.Get(spaceID).Do()
		if err != nil {
			return toolHint(fmt.Sprintf("getting space info for %s: %v", spaceID, err), `chat_list_spaces() then chat_list_messages(space_id="spaces/…")`), nil
		}
		spaceName := spaceInfo.DisplayName
		if spaceName == "" {
			spaceName = "Unknown Space"
		}

		// Get messages
		resp, err := svc.Spaces.Messages.List(spaceID).
			PageSize(int64(pageSize)).
			OrderBy(orderBy).
			Do()
		if err != nil {
			return toolHint(fmt.Sprintf("listing messages: %v", err), `chat_list_spaces() then chat_list_messages(space_id="spaces/…")`), nil
		}

		messages := resp.Messages
		if len(messages) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No messages found in space '%s' (ID: %s).", spaceName, spaceID)), nil
		}

		var out strings.Builder
		fmt.Fprintf(&out, "Messages from '%s' (ID: %s):\n", spaceName, spaceID)
		for _, msg := range messages {
			sender := "Unknown Sender"
			if msg.Sender != nil && msg.Sender.DisplayName != "" {
				sender = msg.Sender.DisplayName
			}
			createTime := msg.CreateTime
			if createTime == "" {
				createTime = "Unknown Time"
			}
			text := msg.Text
			if text == "" {
				text = "No text content"
			}

			fmt.Fprintf(&out, "\n[%s] %s:", createTime, sender)
			fmt.Fprintf(&out, "\n  %s", text)
			fmt.Fprintf(&out, "\n  (Message ID: %s)\n", msg.Name)
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

// --- chat_send_message ---

func registerSendMessage(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("chat_send_message",
		mcp.WithDescription("Send a message to space_id. Confirm unless user asked to send. Required: space_id from chat_list_spaces (spaces/…). Read history → chat_list_messages."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("space_id", mcp.Required(), mcp.Description("Chat space resource name from chat_list_spaces (spaces/…). Not a display name.")),
		mcp.WithString("message_text", mcp.Required(), mcp.Description("Message body text.")),
	)
	s.AddTool(tool, handleSendMessage(getClient))
}

func handleSendMessage(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `chat_send_message(space_id="spaces/…", message_text=…)`), nil
		}
		spaceID, err := request.RequireString("space_id")
		if err != nil || strings.TrimSpace(spaceID) == "" {
			return needArg("space_id", `chat_list_spaces() then chat_send_message(space_id="spaces/…", message_text=…)`), nil
		}
		spaceID = normalizeChatSpaceID(spaceID)

		messageText, err := request.RequireString("message_text")
		if err != nil || strings.TrimSpace(messageText) == "" {
			return needArg("message_text", `chat_send_message(space_id="spaces/…", message_text=…)`), nil
		}

		// Optional thread_key still accepted if a client sends it (not in lean schema).
		threadKey := request.GetString("thread_key", "")

		svc, err := newChatService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		msgBody := &chat.Message{
			Text: messageText,
		}

		call := svc.Spaces.Messages.Create(spaceID, msgBody)
		if threadKey != "" {
			call = call.ThreadKey(threadKey)
		}

		message, err := call.Do()
		if err != nil {
			return toolHint(fmt.Sprintf("sending message: %v", err), `chat_list_spaces() then chat_send_message(space_id="spaces/…", message_text=…)`), nil
		}

		result := fmt.Sprintf("Message sent to space '%s' by %s. Message ID: %s, Time: %s",
			spaceID, email, message.Name, message.CreateTime)

		return mcp.NewToolResultText(result), nil
	}
}

// --- chat_search_messages ---

func registerSearchMessages(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := newMCPTool("chat_search_messages",
		mcp.WithDescription("Search Chat messages by text. Optional space_id (spaces/… from chat_list_spaces). Full thread in one space → chat_list_messages."),
		mcp.WithString("user_google_email", mcp.Description("User Google email (or set USER_GOOGLE_EMAIL).")),
		mcp.WithString("query", mcp.Required(), mcp.Description("Text to find in messages.")),
		mcp.WithString("space_id", mcp.Description("Optional space resource name (spaces/…) to limit search.")),
	)
	s.AddTool(tool, handleSearchMessages(getClient))
}

func handleSearchMessages(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return needArg("user_google_email", `chat_search_messages(query="…")`), nil
		}
		query, err := request.RequireString("query")
		if err != nil || strings.TrimSpace(query) == "" {
			return needArg("query", `chat_search_messages(query="…")`), nil
		}
		query = strings.TrimSpace(query)

		spaceID := request.GetString("space_id", "")
		if strings.TrimSpace(spaceID) != "" {
			spaceID = normalizeChatSpaceID(spaceID)
		}
		// Optional page_size still accepted if a client sends it (not in lean schema).
		pageSize := request.GetInt("page_size", 25)

		svc, err := newChatService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var results []messageWithSpace
		var searchContext string

		filter := fmt.Sprintf(`text:"%s"`, query)

		if spaceID != "" {
			resp, err := svc.Spaces.Messages.List(spaceID).PageSize(int64(pageSize)).Filter(filter).Do()
			if err != nil {
				return toolHint(fmt.Sprintf("searching messages: %v", err), `chat_list_spaces() then chat_search_messages(query="…", space_id="spaces/…")`), nil
			}
			results = messagesWithSpace(resp.Messages, "")
			searchContext = fmt.Sprintf("space '%s'", spaceID)
		} else {
			results, err = searchMessagesInSpaces(svc, filter)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			searchContext = "all accessible spaces"
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No messages found matching '%s' in %s.", query, searchContext)), nil
		}

		var out strings.Builder
		fmt.Fprintf(&out, "Found %d messages matching '%s' in %s:", len(results), query, searchContext)
		for _, r := range results {
			sender := "Unknown Sender"
			if r.msg.Sender != nil && r.msg.Sender.DisplayName != "" {
				sender = r.msg.Sender.DisplayName
			}
			createTime := r.msg.CreateTime
			if createTime == "" {
				createTime = "Unknown Time"
			}
			text := r.msg.Text
			if text == "" {
				text = "No text content"
			}
			spaceName := r.spaceName
			if spaceName == "" {
				spaceName = "Unknown Space"
			}

			// Truncate long messages
			if len(text) > 100 {
				text = text[:100] + "..."
			}

			fmt.Fprintf(&out, "\n- [%s] %s in '%s': %s", createTime, sender, spaceName, text)
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

type messageWithSpace struct {
	msg       *chat.Message
	spaceName string
}

func searchMessagesInSpaces(svc *chat.Service, filter string) ([]messageWithSpace, error) {
	spacesResp, err := svc.Spaces.List().PageSize(100).Do()
	if err != nil {
		return nil, fmt.Errorf("listing spaces for search: %w", err)
	}
	spaces := spacesResp.Spaces
	if len(spaces) > 10 {
		spaces = spaces[:10]
	}
	var results []messageWithSpace
	for _, space := range spaces {
		resp, err := svc.Spaces.Messages.List(space.Name).PageSize(5).Filter(filter).Do()
		if err != nil {
			continue
		}
		results = append(results, messagesWithSpace(resp.Messages, valueOrDefault(space.DisplayName, "Unknown"))...)
	}
	return results, nil
}

func messagesWithSpace(messages []*chat.Message, spaceName string) []messageWithSpace {
	results := make([]messageWithSpace, 0, len(messages))
	for _, msg := range messages {
		results = append(results, messageWithSpace{msg: msg, spaceName: spaceName})
	}
	return results
}
