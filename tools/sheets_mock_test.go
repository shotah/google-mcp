package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// sheetsTestServer creates an MCP server with specific Sheets tools registered,
// backed by the given fake HTTP server.
func sheetsTestServer(t *testing.T, registerFuncs []func(s *mcpserver.MCPServer, getClient httpClientFunc), getClient httpClientFunc) *mcpserver.MCPServer {
	t.Helper()
	t.Setenv("USER_GOOGLE_EMAIL", "test@example.com")
	t.Setenv("WORKSPACE_MCP_CREDENTIALS_DIR", t.TempDir())
	s := mcpserver.NewMCPServer("test", "0.0.0")
	for _, reg := range registerFuncs {
		reg(s, getClient)
	}
	return s
}

// --- sheets_list_spreadsheets ---

func TestSheetsMockListSpreadsheets(t *testing.T) {
	t.Run("success_with_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"ss001","name":"Budget 2026","modifiedTime":"2026-02-10T08:00:00Z","webViewLink":"https://docs.google.com/spreadsheets/d/ss001/edit"},
					{"id":"ss002","name":"Inventory","modifiedTime":"2026-02-05T14:00:00Z","webViewLink":"https://docs.google.com/spreadsheets/d/ss002/edit"}
				]
			}`,
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListSpreadsheets}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_list_spreadsheets", map[string]any{
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Successfully listed 2 spreadsheets") {
			t.Errorf("expected 2 spreadsheets listed, got:\n%s", text)
		}
		if !strings.Contains(text, "Budget 2026") {
			t.Errorf("expected 'Budget 2026' in output")
		}
		if !strings.Contains(text, "ss001") {
			t.Errorf("expected spreadsheet ID in output")
		}
	})

	t.Run("success_no_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerListSpreadsheets}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_list_spreadsheets", map[string]any{
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "No spreadsheets found") {
			t.Errorf("expected 'No spreadsheets found', got:\n%s", text)
		}
	})
}

// --- sheets_create_spreadsheet ---

func TestSheetsMockCreateSpreadsheet(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v4/spreadsheets": map[string]any{
				"spreadsheetId":  "newss001",
				"spreadsheetUrl": "https://docs.google.com/spreadsheets/d/newss001/edit",
				"properties": map[string]any{
					"title":  "New Budget",
					"locale": "en_US",
				},
			},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerCreateSpreadsheet}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_create_spreadsheet", map[string]any{
			"title":             "New Budget",
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Successfully created spreadsheet") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "newss001") {
			t.Errorf("expected spreadsheet ID in output")
		}
		if !strings.Contains(text, "New Budget") {
			t.Errorf("expected title in output")
		}
	})
}

// --- sheets_get_spreadsheet_info ---

func TestSheetsMockGetSpreadsheetInfo(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v4/spreadsheets/ss001": map[string]any{
				"spreadsheetId": "ss001",
				"properties": map[string]any{
					"title":  "Sales Report",
					"locale": "en_US",
				},
				"sheets": []map[string]any{
					{
						"properties": map[string]any{
							"sheetId": 0,
							"title":   "Q1",
							"gridProperties": map[string]any{
								"rowCount":    100,
								"columnCount": 26,
							},
						},
					},
					{
						"properties": map[string]any{
							"sheetId": 1,
							"title":   "Q2",
							"gridProperties": map[string]any{
								"rowCount":    200,
								"columnCount": 10,
							},
						},
					},
				},
			},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerGetSpreadsheetInfo}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_get_spreadsheet_info", map[string]any{
			"spreadsheet_id":    "ss001",
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Sales Report") {
			t.Errorf("expected spreadsheet title in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Q1") {
			t.Errorf("expected sheet name Q1 in output")
		}
		if !strings.Contains(text, "Q2") {
			t.Errorf("expected sheet name Q2 in output")
		}
		if !strings.Contains(text, "Sheets (2)") {
			t.Errorf("expected 'Sheets (2)' in output, got:\n%s", text)
		}
	})
}

// --- sheets_read_values ---

func TestSheetsMockReadSheetValues(t *testing.T) {
	t.Run("success_with_data", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v4/spreadsheets/ss001/values/": map[string]any{
				"range": "Sheet1!A1:C3",
				"values": [][]any{
					{"Name", "Age", "City"},
					{"Alice", 30, "NYC"},
					{"Bob", 25, "LA"},
				},
			},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerReadSheetValues}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_read_values", map[string]any{
			"spreadsheet_id":    "ss001",
			"range_name":        "Sheet1!A1:C3",
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Successfully read") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "3 rows") {
			t.Errorf("expected '3 rows' in output, got:\n%s", text)
		}
	})

	t.Run("success_no_data", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v4/spreadsheets/ss001/values/": map[string]any{
				"range": "Sheet1!A1:C3",
			},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerReadSheetValues}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_read_values", map[string]any{
			"spreadsheet_id":    "ss001",
			"range_name":        "Sheet1!A1:C3",
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "No data found") {
			t.Errorf("expected 'No data found', got:\n%s", text)
		}
	})
}

// --- sheets_modify_values ---

func TestSheetsMockModifySheetValues(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v4/spreadsheets/ss001/values/": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"updatedRange":"Sheet1!A1:B2","updatedRows":2,"updatedColumns":2,"updatedCells":4}`)
			},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerModifySheetValues}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_modify_values", map[string]any{
			"spreadsheet_id":    "ss001",
			"range_name":        "Sheet1!A1:B2",
			"values":            []any{[]any{"a", "b"}, []any{"c", "d"}},
			"user_google_email": "test@example.com",
		})
		if isError {
			t.Fatalf("unexpected error: %s", text)
		}
		if !strings.Contains(text, "Updated") || !strings.Contains(text, "cells") {
			t.Errorf("expected update confirmation, got:\n%s", text)
		}
	})
}

// --- sheets_format_range ---

func TestSheetsMockFormatSheetRange(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/v4/spreadsheets/ss001": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"sheets":[{"properties":{"sheetId":0,"title":"Budget"}}]}`)
		},
		"/v4/spreadsheets/ss001:batchUpdate": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"replies":[{}]}`)
		},
	})
	s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerFormatSheetRange}, testClientFunc(ts))
	text, isError := callTool(t, s, "sheets_format_range", map[string]any{
		"spreadsheet_id":     "ss001",
		"range_name":         "Budget!A1:B2",
		"background_color":   "#FFEECC",
		"number_format_type": "CURRENCY",
		"user_google_email":  "test@example.com",
	})
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Applied formatting") || !strings.Contains(text, "background #FFEECC") {
		t.Errorf("expected formatting confirmation, got:\n%s", text)
	}
}

// --- sheets_add_conditional_formatting ---

func TestSheetsMockAddConditionalFormatting(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/v4/spreadsheets/ss001": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"sheets":[{"properties":{"sheetId":0,"title":"Budget"},"conditionalFormats":[]}]}`)
		},
		"/v4/spreadsheets/ss001:batchUpdate": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"replies":[{}]}`)
		},
	})
	s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerAddConditionalFormatting}, testClientFunc(ts))
	text, isError := callTool(t, s, "sheets_add_conditional_formatting", map[string]any{
		"spreadsheet_id":    "ss001",
		"range_name":        "Budget!B2:B10",
		"condition_type":    "NUMBER_GREATER",
		"condition_values":  `["100"]`,
		"background_color":  "#00FF00",
		"user_google_email": "test@example.com",
	})
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Added conditional format") || !strings.Contains(text, "NUMBER_GREATER") {
		t.Errorf("expected conditional formatting confirmation, got:\n%s", text)
	}
}

func TestSheetsMockCreateSheet(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v4/spreadsheets/ss001:batchUpdate": `{
			"replies":[{"addSheet":{"properties":{"sheetId":7,"title":"New Tab"}}}]
		}`,
	})
	s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerCreateSheet}, testClientFunc(ts))
	text, isError := callTool(t, s, "sheets_create_sheet", map[string]any{
		"spreadsheet_id":    "ss001",
		"sheet_name":        "New Tab",
		"user_google_email": "test@example.com",
	})
	if isError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Successfully created sheet 'New Tab'") || !strings.Contains(text, "ID: 7") {
		t.Errorf("unexpected create sheet output:\n%s", text)
	}
}

// --- API error responses ---

func TestSheetsMockAPIError(t *testing.T) {
	t.Run("get_spreadsheet_info_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v4/spreadsheets/ss001": {code: 404, body: `{"error": {"code": 404, "message": "Not Found"}}`},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerGetSpreadsheetInfo}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_get_spreadsheet_info", map[string]any{
			"spreadsheet_id":    "ss001",
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error, got success: %s", text)
		}
		if !strings.Contains(text, "getting spreadsheet info") {
			t.Errorf("expected error about getting spreadsheet info, got:\n%s", text)
		}
	})

	t.Run("create_spreadsheet_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v4/spreadsheets": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		s := sheetsTestServer(t, []func(*mcpserver.MCPServer, httpClientFunc){registerCreateSpreadsheet}, testClientFunc(ts))
		text, isError := callTool(t, s, "sheets_create_spreadsheet", map[string]any{
			"title":             "Bad Sheet",
			"user_google_email": "test@example.com",
		})
		if !isError {
			t.Fatalf("expected error, got success: %s", text)
		}
		if !strings.Contains(text, "creating spreadsheet") {
			t.Errorf("expected error about creating spreadsheet, got:\n%s", text)
		}
	})
}
