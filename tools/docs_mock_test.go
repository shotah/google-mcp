package tools

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// --- docs_search ---

func TestDocsMockSearchDocs(t *testing.T) {
	t.Run("success_with_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"doc001","name":"Meeting Notes","createdTime":"2026-01-10T10:00:00Z","modifiedTime":"2026-02-15T08:00:00Z","webViewLink":"https://docs.google.com/document/d/doc001/edit"},
					{"id":"doc002","name":"Project Plan","createdTime":"2026-01-05T12:00:00Z","modifiedTime":"2026-02-10T14:00:00Z","webViewLink":"https://docs.google.com/document/d/doc002/edit"}
				]
			}`,
		})
		handler := handleSearchDocs(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "project",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 Google Docs") {
			t.Errorf("expected 'Found 2 Google Docs', got:\n%s", text)
		}
		if !strings.Contains(text, "Meeting Notes") {
			t.Errorf("expected 'Meeting Notes' in output")
		}
		if !strings.Contains(text, "doc001") {
			t.Errorf("expected doc001 in output")
		}
	})

	t.Run("success_no_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := handleSearchDocs(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "nonexistent",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No Google Docs found") {
			t.Errorf("expected 'No Google Docs found', got:\n%s", text)
		}
	})
}

// --- docs_create ---

func TestDocsMockCreateDoc(t *testing.T) {
	t.Run("success_with_content", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/v1/documents": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost && !strings.Contains(r.URL.Path, ":batchUpdate") {
					fmt.Fprint(w, `{"documentId":"newdoc001","title":"My New Doc"}`)
				}
			},
			"/v1/documents/newdoc001:batchUpdate": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"replies":[]}`)
			},
		})
		handler := handleCreateDoc(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"title":             "My New Doc",
			"content":           "Initial content here",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Created Google Doc") {
			t.Errorf("expected 'Created Google Doc', got:\n%s", text)
		}
		if !strings.Contains(text, "My New Doc") {
			t.Errorf("expected doc title in output")
		}
		if !strings.Contains(text, "newdoc001") {
			t.Errorf("expected doc ID in output")
		}
	})

	t.Run("success_without_content", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/documents": map[string]any{
				"documentId": "newdoc002",
				"title":      "Empty Doc",
			},
		})
		handler := handleCreateDoc(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"title":             "Empty Doc",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Created Google Doc") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "newdoc002") {
			t.Errorf("expected doc ID in output")
		}
	})
}

// --- docs_get_content ---

func TestDocsMockGetDocContent(t *testing.T) {
	t.Run("success_google_doc", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/doc001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"doc001","name":"My Document","mimeType":"application/vnd.google-apps.document","webViewLink":"https://docs.google.com/document/d/doc001/edit"}`)
			},
			"/v1/documents/doc001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"documentId":"doc001",
					"title":"My Document",
					"body":{
						"content":[
							{"startIndex":0,"endIndex":1,"sectionBreak":{}},
							{"startIndex":1,"endIndex":14,"paragraph":{
								"elements":[{"startIndex":1,"endIndex":14,"textRun":{"content":"Hello World!\n"}}]
							}}
						]
					}
				}`)
			},
		})
		handler := handleGetDocContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"document_id":       "doc001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "My Document") {
			t.Errorf("expected document name in output, got:\n%s", text)
		}
	})

	t.Run("success_plain_text_file", func(t *testing.T) {
		fileContent := "Plain text file content"
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("alt") == "media" {
					w.Header().Set("Content-Type", "text/plain")
					fmt.Fprint(w, fileContent)
				} else {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"file001","name":"readme.txt","mimeType":"text/plain","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
				}
			},
		})
		handler := handleGetDocContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"document_id":       "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "readme.txt") {
			t.Errorf("expected file name in output, got:\n%s", text)
		}
		if !strings.Contains(text, fileContent) {
			t.Errorf("expected file content in output, got:\n%s", text)
		}
	})
}

// --- docs_list_in_folder ---

func TestDocsMockListDocsInFolder(t *testing.T) {
	t.Run("success_with_docs", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"doc001","name":"Readme.gdoc","modifiedTime":"2026-02-01T08:00:00Z","webViewLink":"https://docs.google.com/document/d/doc001/edit"}
				]
			}`,
		})
		handler := handleListDocsInFolder(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"folder_id":         "folder001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 1 Docs") {
			t.Errorf("expected 'Found 1 Docs', got:\n%s", text)
		}
		if !strings.Contains(text, "Readme.gdoc") {
			t.Errorf("expected doc name in output")
		}
	})

	t.Run("success_no_docs", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := handleListDocsInFolder(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No Google Docs found") {
			t.Errorf("expected 'No Google Docs found', got:\n%s", text)
		}
	})
}

// --- docs_inspect_structure ---

func TestDocsMockInspectDocStructure(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/documents/doc001": `{
			"documentId":"doc001",
			"title":"Architecture Notes",
			"body":{"content":[
				{"startIndex":0,"endIndex":1,"sectionBreak":{}},
				{"startIndex":1,"endIndex":20,"paragraph":{"elements":[
					{"startIndex":1,"endIndex":20,"textRun":{"content":"System overview\n"}}
				]}}
			]}
		}`,
	})
	handler := handleInspectDocStructure(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Document structure analysis") {
		t.Errorf("expected structure analysis output, got:\n%s", text)
	}
	if !strings.Contains(text, "Architecture Notes") || !strings.Contains(text, `"paragraphs": 1`) {
		t.Errorf("expected document structure details, got:\n%s", text)
	}
}

// --- docs_modify_text ---

func TestDocsMockModifyDocText(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/documents/doc001:batchUpdate": `{"replies":[]}`,
	})
	handler := handleModifyDocText(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"start_index":       1,
		"text":              "Updated introduction",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Inserted text at index 1") {
		t.Errorf("expected insertion confirmation, got:\n%s", text)
	}
	if !strings.Contains(text, "doc001") {
		t.Errorf("expected document ID in output, got:\n%s", text)
	}
}

// --- docs_find_and_replace ---

func TestDocsMockFindAndReplaceDoc(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/documents/doc001:batchUpdate": `{
			"replies":[{"replaceAllText":{"occurrencesChanged":2}}]
		}`,
	})
	handler := handleFindAndReplaceDoc(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"find_text":         "draft",
		"replace_text":      "final",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Replaced 2 occurrence(s)") {
		t.Errorf("expected replacement count, got:\n%s", text)
	}
	if !strings.Contains(text, "'draft' with 'final'") {
		t.Errorf("expected replacement details, got:\n%s", text)
	}
}

func TestDocsMockBatchUpdateDoc(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/documents/doc001:batchUpdate": `{"replies":[{"insertText":{}}]}`,
	})
	handler := handleBatchUpdateDoc(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"document_id": "doc001",
		"operations": []any{
			map[string]any{"type": "insert_text", "index": 1, "text": "Hello"},
		},
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Successfully executed 1 operations") ||
		!strings.Contains(text, "insert text at 1") ||
		!strings.Contains(text, "API replies: 1") {
		t.Errorf("expected batch update result, got:\n%s", text)
	}
}

func TestDocsMockExportDocToPDF(t *testing.T) {
	uploadResp := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"pdf001","name":"Report_PDF.pdf","webViewLink":"https://drive.google.com/file/d/pdf001/view","parents":["root"]}`)
	}
	ts := driveFakeServer(t, map[string]any{
		"/drive/v3/files/doc001/export": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/pdf")
			fmt.Fprint(w, "%PDF-1.4 mock")
		},
		"/upload/drive/v3/files": uploadResp,
		"/drive/v3/files":        uploadResp,
		"/drive/v3/files/doc001": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"doc001","name":"Report","mimeType":"application/vnd.google-apps.document","webViewLink":"https://docs.google.com/document/d/doc001/edit"}`)
		},
	})
	handler := handleExportDocToPDF(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"document_id":       "doc001",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Successfully exported") || !strings.Contains(text, "pdf001") {
		t.Errorf("unexpected export output:\n%s", text)
	}
}

// --- API error responses ---

func TestDocsMockAPIError(t *testing.T) {
	t.Run("search_docs_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/drive/v3/files": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := handleSearchDocs(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"query":             "test",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Drive API error") {
			t.Errorf("expected 'Drive API error', got:\n%s", text)
		}
	})

	t.Run("create_doc_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v1/documents": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleCreateDoc(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"title":             "Bad Doc",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Docs API error") {
			t.Errorf("expected 'Docs API error', got:\n%s", text)
		}
	})
}
