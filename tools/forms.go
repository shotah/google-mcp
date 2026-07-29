package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	forms "google.golang.org/api/forms/v1"

	"github.com/shotah/google-mcp/internal/google"
	"github.com/shotah/google-mcp/server"
)

// RegisterFormsTools registers all Forms tools with the MCP server.
func RegisterFormsTools(s *mcpserver.MCPServer, _ server.Config) {
	getClient := clientFuncFromCache(google.DefaultClientCache())

	registerCreateForm(s, getClient)
	registerGetForm(s, getClient)
	registerSetPublishSettings(s, getClient)
	registerGetFormResponse(s, getClient)
	registerListFormResponses(s, getClient)
	registerBatchUpdateForm(s, getClient)
}

// newFormsService creates a forms.Service for the given user email.
func newFormsService(ctx context.Context, getClient httpClientFunc, email string) (*forms.Service, error) {
	httpClient, err := getClient(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating for %s: %w", email, err)
	}
	svc, err := forms.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating Forms service: %w", err)
	}
	return svc, nil
}

// --- forms_create ---

func registerCreateForm(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_create",
		mcp.WithDescription("Create a new Google Form (title, optional description). Returns form_id and URLs. Use for 'make a survey/form'. Edit questions with forms_batch_update."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("The title of the form.")),
		mcp.WithString("description", mcp.Description("The description of the form.")),
		mcp.WithString("document_title", mcp.Description("The document title (shown in browser tab).")),
	)
	s.AddTool(tool, handleCreateForm(getClient))
}

func handleCreateForm(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		title, err := request.RequireString("title")
		if err != nil {
			return mcp.NewToolResultError("title is required"), nil
		}

		description := request.GetString("description", "")
		documentTitle := request.GetString("document_title", "")

		svc, err := newFormsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		formBody := &forms.Form{
			Info: &forms.Info{
				Title: title,
			},
		}

		if description != "" {
			formBody.Info.Description = description
		}

		if documentTitle != "" {
			formBody.Info.DocumentTitle = documentTitle
		}

		created, err := svc.Forms.Create(formBody).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating form: %v", err)), nil
		}

		formID := created.FormId
		editURL := fmt.Sprintf("https://docs.google.com/forms/d/%s/edit", formID)
		responderURL := created.ResponderUri
		if responderURL == "" {
			responderURL = fmt.Sprintf("https://docs.google.com/forms/d/%s/viewform", formID)
		}

		createdTitle := title
		if created.Info != nil && created.Info.Title != "" {
			createdTitle = created.Info.Title
		}

		result := fmt.Sprintf("Successfully created form '%s' for %s. Form ID: %s. Edit URL: %s. Responder URL: %s",
			createdTitle, email, formID, editURL, responderURL)

		return mcp.NewToolResultText(result), nil
	}
}

// --- forms_get ---

func registerGetForm(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_get",
		mcp.WithDescription("READ form structure: title, questions, settings, responder URLs. Required form_id. Use before forms_batch_update or listing responses. Not for answers — use forms_list_responses."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("form_id", mcp.Required(), mcp.Description("The ID of the form to retrieve.")),
	)
	s.AddTool(tool, handleGetForm(getClient))
}

func handleGetForm(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		formID, err := request.RequireString("form_id")
		if err != nil {
			return mcp.NewToolResultError("form_id is required"), nil
		}

		svc, err := newFormsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		form, err := svc.Forms.Get(formID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting form: %v", err)), nil
		}

		title := "No Title"
		description := "No Description"
		documentTitle := title
		if form.Info != nil {
			if form.Info.Title != "" {
				title = form.Info.Title
			}
			if form.Info.Description != "" {
				description = form.Info.Description
			}
			if form.Info.DocumentTitle != "" {
				documentTitle = form.Info.DocumentTitle
			} else {
				documentTitle = title
			}
		}

		editURL := fmt.Sprintf("https://docs.google.com/forms/d/%s/edit", formID)
		responderURL := form.ResponderUri
		if responderURL == "" {
			responderURL = fmt.Sprintf("https://docs.google.com/forms/d/%s/viewform", formID)
		}

		items := form.Items
		var questionsSummary []string
		for i, item := range items {
			itemTitle := item.Title
			if itemTitle == "" {
				itemTitle = fmt.Sprintf("Question %d", i+1)
			}
			requiredText := ""
			if item.QuestionItem != nil && item.QuestionItem.Question != nil && item.QuestionItem.Question.Required {
				requiredText = " (Required)"
			}
			questionsSummary = append(questionsSummary, fmt.Sprintf("  %d. %s%s", i+1, itemTitle, requiredText))
		}

		questionsText := "  No questions found"
		if len(questionsSummary) > 0 {
			questionsText = strings.Join(questionsSummary, "\n")
		}

		result := fmt.Sprintf(`Form Details for %s:
- Title: "%s"
- Description: "%s"
- Document Title: "%s"
- Form ID: %s
- Edit URL: %s
- Responder URL: %s
- Questions (%d total):
%s`, email, title, description, documentTitle, formID, editURL, responderURL, len(items), questionsText)

		return mcp.NewToolResultText(result), nil
	}
}

// --- forms_set_publish_settings ---

func registerSetPublishSettings(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_set_publish_settings",
		mcp.WithDescription("Change publish/accept-response settings for form_id. Use for 'open/close the form'. Not for editing questions — use forms_batch_update."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("form_id", mcp.Required(), mcp.Description("The ID of the form to update publish settings for.")),
		mcp.WithBoolean("publish_as_template", mcp.Description("Whether to publish as a template. Defaults to false.")),
		mcp.WithBoolean("require_authentication", mcp.Description("Whether to require authentication to view/submit. Defaults to false.")),
	)
	s.AddTool(tool, handleSetPublishSettings(getClient))
}

func handleSetPublishSettings(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		formID, err := request.RequireString("form_id")
		if err != nil {
			return mcp.NewToolResultError("form_id is required"), nil
		}

		publishAsTemplate := getBool(request, "publish_as_template", false)
		requireAuthentication := getBool(request, "require_authentication", false)

		// The setPublishSettings endpoint is not in the generated Go Forms client library.
		// We make a raw HTTP call to the REST API.
		httpClient, err := getClient(ctx, email)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("authenticating for %s: %v", email, err)), nil
		}

		settingsBody := map[string]bool{
			"publishAsTemplate":     publishAsTemplate,
			"requireAuthentication": requireAuthentication,
		}
		bodyJSON, err := json.Marshal(settingsBody)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling request body: %v", err)), nil
		}

		url := fmt.Sprintf("https://forms.googleapis.com/v1/forms/%s:setPublishSettings", formID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating request: %v", err)), nil
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("setting publish settings: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return mcp.NewToolResultError(fmt.Sprintf("setting publish settings (HTTP %d): %s", resp.StatusCode, string(respBody))), nil
		}

		result := fmt.Sprintf("Successfully updated publish settings for form %s for %s. Publish as template: %t, Require authentication: %t",
			formID, email, publishAsTemplate, requireAuthentication)

		return mcp.NewToolResultText(result), nil
	}
}

// --- forms_get_response ---

func registerGetFormResponse(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_get_response",
		mcp.WithDescription("Get one form submission by form_id + response_id. Use after forms_list_responses when you need full answers for one response."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("form_id", mcp.Required(), mcp.Description("The ID of the form.")),
		mcp.WithString("response_id", mcp.Required(), mcp.Description("The ID of the response to retrieve.")),
	)
	s.AddTool(tool, handleGetFormResponse(getClient))
}

func handleGetFormResponse(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		formID, err := request.RequireString("form_id")
		if err != nil {
			return mcp.NewToolResultError("form_id is required"), nil
		}
		responseID, err := request.RequireString("response_id")
		if err != nil {
			return mcp.NewToolResultError("response_id is required"), nil
		}

		svc, err := newFormsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		formResp, err := svc.Forms.Responses.Get(formID, responseID).Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("getting form response: %v", err)), nil
		}

		respID := formResp.ResponseId
		if respID == "" {
			respID = "Unknown"
		}
		createTime := formResp.CreateTime
		if createTime == "" {
			createTime = "Unknown"
		}
		lastSubmittedTime := formResp.LastSubmittedTime
		if lastSubmittedTime == "" {
			lastSubmittedTime = "Unknown"
		}

		var answerDetails []string
		for questionID, answer := range formResp.Answers {
			if answer.TextAnswers != nil && len(answer.TextAnswers.Answers) > 0 {
				var vals []string
				for _, ans := range answer.TextAnswers.Answers {
					vals = append(vals, ans.Value)
				}
				answerDetails = append(answerDetails, fmt.Sprintf("  Question ID %s: %s", questionID, strings.Join(vals, ", ")))
			} else {
				answerDetails = append(answerDetails, fmt.Sprintf("  Question ID %s: No answer provided", questionID))
			}
		}

		answersText := "  No answers found"
		if len(answerDetails) > 0 {
			answersText = strings.Join(answerDetails, "\n")
		}

		result := fmt.Sprintf(`Form Response Details for %s:
- Form ID: %s
- Response ID: %s
- Created: %s
- Last Submitted: %s
- Answers:
%s`, email, formID, respID, createTime, lastSubmittedTime, answersText)

		return mcp.NewToolResultText(result), nil
	}
}

// --- forms_list_responses ---

func registerListFormResponses(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_list_responses",
		mcp.WithDescription("List form responses for form_id (paginated). Use for 'show survey results', export answers. Prefer forms_get_response for one submission."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("form_id", mcp.Required(), mcp.Description("The ID of the form.")),
		mcp.WithNumber("page_size", mcp.Description("Maximum number of responses to return. Defaults to 10.")),
		mcp.WithString("page_token", mcp.Description("Token for retrieving next page of results.")),
	)
	s.AddTool(tool, handleListFormResponses(getClient))
}

func handleListFormResponses(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		formID, err := request.RequireString("form_id")
		if err != nil {
			return mcp.NewToolResultError("form_id is required"), nil
		}

		pageSize := request.GetInt("page_size", 10)
		pageToken := request.GetString("page_token", "")

		svc, err := newFormsService(ctx, getClient, email)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		call := svc.Forms.Responses.List(formID).PageSize(int64(pageSize))
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("listing form responses: %v", err)), nil
		}

		responses := resp.Responses
		if len(responses) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No responses found for form %s for %s.", formID, email)), nil
		}

		var responseDetails []string
		for i, r := range responses {
			respID := r.ResponseId
			if respID == "" {
				respID = "Unknown"
			}
			createTime := r.CreateTime
			if createTime == "" {
				createTime = "Unknown"
			}
			lastSubmittedTime := r.LastSubmittedTime
			if lastSubmittedTime == "" {
				lastSubmittedTime = "Unknown"
			}
			answersCount := len(r.Answers)
			responseDetails = append(responseDetails, fmt.Sprintf("  %d. Response ID: %s | Created: %s | Last Submitted: %s | Answers: %d",
				i+1, respID, createTime, lastSubmittedTime, answersCount))
		}

		paginationInfo := "\nNo more pages."
		if resp.NextPageToken != "" {
			paginationInfo = "\nNext page token: " + resp.NextPageToken
		}

		result := fmt.Sprintf(`Form Responses for %s:
- Form ID: %s
- Total responses returned: %d
- Responses:
%s%s`, email, formID, len(responses), strings.Join(responseDetails, "\n"), paginationInfo)

		return mcp.NewToolResultText(result), nil
	}
}

// --- forms_batch_update ---

func registerBatchUpdateForm(s *mcpserver.MCPServer, getClient httpClientFunc) {
	tool := mcp.NewTool("forms_batch_update",
		mcp.WithDescription("Add/update/delete form items and metadata on form_id (questions, sections, settings). Use for 'add a question', restructure form. Prefer forms_get first. Not for reading responses — use forms_list_responses."),
		mcp.WithString("user_google_email", mcp.Description("The user's Google email address.")),
		mcp.WithString("form_id", mcp.Required(), mcp.Description("The ID of the form to update.")),
		mcp.WithArray("requests", mcp.Required(), mcp.Description("List of update requests to apply. Supported types: createItem, updateItem, deleteItem, moveItem, updateFormInfo, updateSettings."), mcp.Items(map[string]any{"type": "object"})),
	)
	s.AddTool(tool, handleBatchUpdateForm(getClient))
}

func handleBatchUpdateForm(getClient httpClientFunc) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		email, err := resolveEmail(request)
		if err != nil {
			return mcp.NewToolResultError("user_google_email is required"), nil
		}
		formID, err := request.RequireString("form_id")
		if err != nil {
			return mcp.NewToolResultError("form_id is required"), nil
		}

		// Extract raw requests array — we pass these as raw JSON to the API
		// since the request format is complex and varies by operation type.
		args := request.GetArguments()
		rawRequests, ok := args["requests"]
		if !ok {
			return mcp.NewToolResultError("requests is required"), nil
		}

		requestsList, ok := rawRequests.([]any)
		if !ok {
			return mcp.NewToolResultError("requests must be an array"), nil
		}

		// Use raw HTTP to send the batch update, since the typed Go structs
		// would require converting each request map into the proper struct.
		httpClient, err := getClient(ctx, email)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("authenticating for %s: %v", email, err)), nil
		}

		body := map[string]any{
			"requests": requestsList,
		}
		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshaling request body: %v", err)), nil
		}

		url := fmt.Sprintf("https://forms.googleapis.com/v1/forms/%s:batchUpdate", formID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("creating request: %v", err)), nil
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("batch updating form: %v", err)), nil
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("reading response: %v", err)), nil
		}

		if resp.StatusCode != http.StatusOK {
			return mcp.NewToolResultError(fmt.Sprintf("batch updating form (HTTP %d): %s", resp.StatusCode, string(respBody))), nil
		}

		// Parse the response to extract reply details
		var batchResp struct {
			Replies []map[string]any `json:"replies"`
		}
		if err := json.Unmarshal(respBody, &batchResp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parsing response: %v", err)), nil
		}

		editURL := fmt.Sprintf("https://docs.google.com/forms/d/%s/edit", formID)

		var out strings.Builder
		fmt.Fprintf(&out, "Batch Update Completed:\n")
		fmt.Fprintf(&out, "- Form ID: %s\n", formID)
		fmt.Fprintf(&out, "- URL: %s\n", editURL)
		fmt.Fprintf(&out, "- Requests Applied: %d\n", len(requestsList))
		fmt.Fprintf(&out, "- Replies Received: %d", len(batchResp.Replies))

		if len(batchResp.Replies) > 0 {
			out.WriteString(formatFormUpdateReplies(batchResp.Replies))
		}

		return mcp.NewToolResultText(out.String()), nil
	}
}

func formatFormUpdateReplies(replies []map[string]any) string {
	var out strings.Builder
	out.WriteString("\n\nUpdate Results:")
	for i, reply := range replies {
		createItem, ok := reply["createItem"]
		if !ok {
			fmt.Fprintf(&out, "\n  Request %d: Operation completed", i+1)
			continue
		}
		createMap, _ := createItem.(map[string]any)
		itemID := "Unknown"
		if id, ok := createMap["itemId"]; ok {
			itemID = fmt.Sprintf("%v", id)
		}
		questionInfo := formatFormQuestionIDs(createMap["questionId"])
		fmt.Fprintf(&out, "\n  Request %d: Created item %s%s", i+1, itemID, questionInfo)
	}
	return out.String()
}

func formatFormQuestionIDs(questionIDs any) string {
	qList, ok := questionIDs.([]any)
	if !ok || len(qList) == 0 {
		return ""
	}
	ids := make([]string, 0, len(qList))
	for _, q := range qList {
		ids = append(ids, fmt.Sprintf("%v", q))
	}
	return fmt.Sprintf(" (Question IDs: %s)", strings.Join(ids, ", "))
}
