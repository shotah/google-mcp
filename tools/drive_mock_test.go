package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// driveFakeServer creates a test server that routes Drive API requests using
// longest-prefix-first matching. Routes can map to:
//   - func(http.ResponseWriter, *http.Request): full control over response
//   - int: HTTP status code with empty body
//   - string: served as-is with Content-Type application/json
//   - any other value: JSON-marshalled and served
//
// Longest-prefix-first matching ensures that "/drive/v3/files/id001/export"
// matches before "/drive/v3/files/id001" and "/drive/v3/files".
func driveFakeServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()
	// Sort prefixes by length descending so longer prefixes match first.
	prefixes := make([]string, 0, len(routes))
	for p := range routes {
		prefixes = append(prefixes, p)
	}
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i]) > len(prefixes[j])
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range prefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				resp := routes[prefix]
				switch v := resp.(type) {
				case int:
					w.WriteHeader(v)
				case string:
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, v)
				case func(http.ResponseWriter, *http.Request):
					v(w, r)
				default:
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(v); err != nil {
						t.Errorf("driveFakeServer: encode response for %s: %v", prefix, err)
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
				return
			}
		}
		t.Logf("driveFakeServer: unmatched: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// --- drive_search_files ---

func TestDriveMockSearchFiles(t *testing.T) {
	t.Run("success_with_results", func(t *testing.T) {
		// File.Size has json:",string" tag so must be a string in JSON.
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"file001","name":"Document.docx","mimeType":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","webViewLink":"https://drive.google.com/file/d/file001/view","modifiedTime":"2026-01-15T10:30:00Z","size":"12345"},
					{"id":"file002","name":"Spreadsheet.xlsx","mimeType":"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet","webViewLink":"https://drive.google.com/file/d/file002/view","modifiedTime":"2026-01-16T14:00:00Z","size":"67890"}
				]
			}`,
		})
		handler := handleSearchDriveFiles(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "test documents",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 files") {
			t.Errorf("expected 'Found 2 files', got:\n%s", text)
		}
		if !strings.Contains(text, "file001") {
			t.Errorf("expected file001 in output")
		}
		if !strings.Contains(text, "Document.docx") {
			t.Errorf("expected Document.docx in output")
		}
		if !strings.Contains(text, "file002") {
			t.Errorf("expected file002 in output")
		}
	})

	t.Run("success_no_results", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := handleSearchDriveFiles(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "nonexistent file",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No files found") {
			t.Errorf("expected 'No files found', got:\n%s", text)
		}
	})

	t.Run("success_structured_query", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"file003","name":"Report.pdf","mimeType":"application/pdf","webViewLink":"https://drive.google.com/file/d/file003/view","modifiedTime":"2026-02-01T08:00:00Z"}
				]
			}`,
		})
		handler := handleSearchDriveFiles(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"query":             "name = 'Report.pdf'",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 1 files") {
			t.Errorf("expected 'Found 1 files', got:\n%s", text)
		}
		if !strings.Contains(text, "Report.pdf") {
			t.Errorf("expected Report.pdf in output")
		}
	})
}

// --- drive_get_file_content ---

func TestDriveMockGetFileContent(t *testing.T) {
	t.Run("success_text_file", func(t *testing.T) {
		fileContent := "Hello, this is the file content."
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
		handler := handleGetDriveFileContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "readme.txt") {
			t.Errorf("expected file name in output, got:\n%s", text)
		}
		if !strings.Contains(text, fileContent) {
			t.Errorf("expected file content in output, got:\n%s", text)
		}
		if !strings.Contains(text, "CONTENT") {
			t.Errorf("expected CONTENT section in output, got:\n%s", text)
		}
	})

	t.Run("success_google_doc_export", func(t *testing.T) {
		exportedContent := "Exported plain text from Google Doc"
		ts := driveFakeServer(t, map[string]any{
			// Export path must be listed separately (longer prefix matches first).
			"/drive/v3/files/doc001/export": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprint(w, exportedContent)
			},
			"/drive/v3/files/doc001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"doc001","name":"My Document","mimeType":"application/vnd.google-apps.document","webViewLink":"https://docs.google.com/document/d/doc001/edit"}`)
			},
		})
		handler := handleGetDriveFileContent(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "doc001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "My Document") {
			t.Errorf("expected document name in output, got:\n%s", text)
		}
		if !strings.Contains(text, exportedContent) {
			t.Errorf("expected exported content in output, got:\n%s", text)
		}
	})
}

// --- drive_create_file ---

func TestDriveMockCreateFile(t *testing.T) {
	t.Run("success_with_content", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/upload/drive/v3/files": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"newfile001","name":"test-file.txt","webViewLink":"https://drive.google.com/file/d/newfile001/view"}`)
			},
		})
		handler := handleCreateDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_name":         "test-file.txt",
			"content":           "Hello, world!",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created file") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "newfile001") {
			t.Errorf("expected file ID in output, got:\n%s", text)
		}
		if !strings.Contains(text, "test-file.txt") {
			t.Errorf("expected file name in output, got:\n%s", text)
		}
	})

	t.Run("error_no_content_or_url", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleCreateDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_name":         "empty.txt",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "content") && !strings.Contains(text, "fileUrl") {
			t.Errorf("expected error about missing content/fileUrl, got:\n%s", text)
		}
	})

	t.Run("success_with_folder", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/folder001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"folder001","name":"My Folder","mimeType":"application/vnd.google-apps.folder"}`)
			},
			"/upload/drive/v3/files": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"newfile002","name":"subfolder-file.txt","webViewLink":"https://drive.google.com/file/d/newfile002/view"}`)
			},
		})
		handler := handleCreateDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_name":         "subfolder-file.txt",
			"content":           "File in a subfolder",
			"folder_id":         "folder001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully created file") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "folder001") {
			t.Errorf("expected folder ID in output, got:\n%s", text)
		}
	})
}

// --- drive_share_file ---

func TestDriveMockShareFile(t *testing.T) {
	t.Run("success_share_with_user", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001/permissions": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"perm001","type":"user","role":"reader","emailAddress":"reader@example.com"}`)
			},
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"file001","name":"SharedDoc.docx","mimeType":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
			},
		})
		handler := handleShareDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"share_with":        "reader@example.com",
			"role":              "reader",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully shared") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "SharedDoc.docx") {
			t.Errorf("expected file name in output, got:\n%s", text)
		}
		if !strings.Contains(text, "reader@example.com") {
			t.Errorf("expected shared email in output, got:\n%s", text)
		}
	})

	t.Run("success_share_with_anyone", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file002/permissions": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"perm002","type":"anyone","role":"reader"}`)
			},
			"/drive/v3/files/file002": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"file002","name":"PublicDoc.pdf","mimeType":"application/pdf","webViewLink":"https://drive.google.com/file/d/file002/view"}`)
			},
		})
		handler := handleShareDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file002",
			"share_type":        "anyone",
			"role":              "reader",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully shared") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Anyone with the link") {
			t.Errorf("expected 'Anyone with the link' in output, got:\n%s", text)
		}
	})

	t.Run("error_missing_share_with_for_user", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleShareDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_id":           "file001",
			"share_type":        "user",
			"role":              "reader",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "share_with is required") {
			t.Errorf("expected share_with error, got:\n%s", text)
		}
	})

	t.Run("error_invalid_role", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{})
		handler := handleShareDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_id":           "file001",
			"share_with":        "user@example.com",
			"role":              "admin",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Invalid role") {
			t.Errorf("expected invalid role error, got:\n%s", text)
		}
	})
}

// --- drive_list_items ---

func TestDriveMockListItems(t *testing.T) {
	t.Run("success_with_items", func(t *testing.T) {
		// File.Size has json:",string" tag — provide as string in raw JSON.
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"file001","name":"notes.txt","mimeType":"text/plain","webViewLink":"https://drive.google.com/file/d/file001/view","modifiedTime":"2026-01-20T09:00:00Z","size":"1024"},
					{"id":"folder001","name":"Subfolder","mimeType":"application/vnd.google-apps.folder","webViewLink":"https://drive.google.com/drive/folders/folder001"}
				]
			}`,
		})
		handler := handleListDriveItems(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 items") {
			t.Errorf("expected 'Found 2 items', got:\n%s", text)
		}
		if !strings.Contains(text, "notes.txt") {
			t.Errorf("expected notes.txt in output")
		}
		if !strings.Contains(text, "Subfolder") {
			t.Errorf("expected Subfolder in output")
		}
	})

	t.Run("success_empty_folder", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := handleListDriveItems(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No items found") {
			t.Errorf("expected 'No items found', got:\n%s", text)
		}
	})
}

// --- drive_get_file_permissions ---

func TestDriveMockGetFilePermissions(t *testing.T) {
	t.Run("success_with_permissions", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// Both resolveDriveItem and main call hit this path.
				// File.Size uses json:",string" — provide as string.
				fmt.Fprint(w, `{
					"id":"file001","name":"Shared Document","mimeType":"application/vnd.google-apps.document",
					"size":"0","modifiedTime":"2026-02-01T10:00:00Z","shared":true,
					"webViewLink":"https://docs.google.com/document/d/file001/edit",
					"permissions":[
						{"id":"perm001","type":"user","role":"owner","emailAddress":"owner@example.com"},
						{"id":"perm002","type":"user","role":"reader","emailAddress":"reader@example.com"},
						{"id":"perm003","type":"anyone","role":"reader"}
					]
				}`)
			},
		})
		handler := handleGetDriveFilePermissions(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Shared Document") {
			t.Errorf("expected file name in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Number of permissions: 3") {
			t.Errorf("expected 3 permissions, got:\n%s", text)
		}
		if !strings.Contains(text, "owner@example.com") {
			t.Errorf("expected owner email in output")
		}
		if !strings.Contains(text, "reader@example.com") {
			t.Errorf("expected reader email in output")
		}
		if !strings.Contains(text, "Anyone with the link") {
			t.Errorf("expected anyone permission in output")
		}
		if !strings.Contains(text, "can be inserted into Google Docs") {
			t.Errorf("expected public access note, got:\n%s", text)
		}
	})

	t.Run("success_private_file", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file002": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"id":"file002","name":"Private File","mimeType":"text/plain",
					"modifiedTime":"2026-01-15T08:00:00Z","shared":false,
					"webViewLink":"https://drive.google.com/file/d/file002/view",
					"permissions":[]
				}`)
			},
		})
		handler := handleGetDriveFilePermissions(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file002",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Private File") {
			t.Errorf("expected file name in output")
		}
		if !strings.Contains(text, "NOT shared") {
			t.Errorf("expected NOT shared note, got:\n%s", text)
		}
	})
}

func TestDriveMockRemovePermission(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/drive/v3/files/file001/permissions/perm002": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		},
		"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"file001","name":"Shared Document","mimeType":"text/plain","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
		},
	})
	handler := handleRemoveDrivePermission(testClientFunc(ts))
	text := callHandlerOK(t, handler, map[string]any{
		"file_id":           "file001",
		"permission_id":     "perm002",
		"user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Shared Document") || !strings.Contains(strings.ToLower(text), "removed") {
		t.Errorf("unexpected remove permission output:\n%s", text)
	}
}

// --- drive_update_file ---

func TestDriveMockUpdateFile(t *testing.T) {
	t.Run("success_rename", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPatch {
					fmt.Fprint(w, `{"id":"file001","name":"Renamed File.txt","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
				} else {
					fmt.Fprint(w, `{"id":"file001","name":"Original File.txt","mimeType":"text/plain","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
				}
			},
		})
		handler := handleUpdateDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"name":              "Renamed File.txt",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully updated file") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "Renamed File.txt") {
			t.Errorf("expected new file name in output, got:\n%s", text)
		}
	})

	t.Run("error_no_updates", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"file001","name":"File.txt","mimeType":"text/plain"}`)
			},
		})
		handler := handleUpdateDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_id":           "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No updates specified") {
			t.Errorf("expected no updates error, got:\n%s", text)
		}
	})
}

// --- drive_copy_file ---

func TestDriveMockCopyFile(t *testing.T) {
	t.Run("success_default_name", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001/copy": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"copy001","name":"Copy of Original.docx","webViewLink":"https://drive.google.com/file/d/copy001/view","mimeType":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","parents":["root"]}`)
			},
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"file001","name":"Original.docx","mimeType":"application/vnd.openxmlformats-officedocument.wordprocessingml.document","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
			},
		})
		handler := handleCopyDriveFile(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Successfully copied") {
			t.Errorf("expected success message, got:\n%s", text)
		}
		if !strings.Contains(text, "copy001") {
			t.Errorf("expected new file ID in output, got:\n%s", text)
		}
		if !strings.Contains(text, "Original.docx") {
			t.Errorf("expected original file name in output, got:\n%s", text)
		}
	})
}

// --- drive_check_file_public_access ---

func TestDriveMockCheckPublicAccess(t *testing.T) {
	t.Run("success_public_file", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/img001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"img001","name":"photo.png","mimeType":"image/png","shared":true,"permissions":[{"id":"perm001","type":"anyone","role":"reader"}]}`)
			},
			"/drive/v3/files": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"files":[{"id":"img001","name":"photo.png","mimeType":"image/png","webViewLink":"https://drive.google.com/file/d/img001/view"}]}`)
			},
		})
		handler := handleCheckDriveFilePublicAccess(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_name":         "photo.png",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "PUBLIC ACCESS ENABLED") {
			t.Errorf("expected PUBLIC ACCESS ENABLED, got:\n%s", text)
		}
	})

	t.Run("success_private_file", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/doc001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"doc001","name":"secret.doc","mimeType":"application/msword","shared":false,"permissions":[]}`)
			},
			"/drive/v3/files": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"files":[{"id":"doc001","name":"secret.doc","mimeType":"application/msword","webViewLink":"https://drive.google.com/file/d/doc001/view"}]}`)
			},
		})
		handler := handleCheckDriveFilePublicAccess(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_name":         "secret.doc",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "NO PUBLIC ACCESS") {
			t.Errorf("expected NO PUBLIC ACCESS, got:\n%s", text)
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := handleCheckDriveFilePublicAccess(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_name":         "nonexistent.pdf",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No file found") {
			t.Errorf("expected no file found message, got:\n%s", text)
		}
	})
}

// --- drive_get_shareable_link ---

func TestDriveMockGetShareableLink(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{
					"id":"file001","name":"Presentation.pptx","mimeType":"application/vnd.google-apps.presentation",
					"shared":true,"webViewLink":"https://docs.google.com/presentation/d/file001/edit",
					"permissions":[{"id":"perm001","type":"user","role":"owner","emailAddress":"owner@example.com"}]
				}`)
			},
		})
		handler := handleGetDriveShareableLink(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"file_id":           "file001",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Presentation.pptx") {
			t.Errorf("expected file name in output")
		}
		if !strings.Contains(text, "https://docs.google.com/presentation/d/file001/edit") {
			t.Errorf("expected view link in output, got:\n%s", text)
		}
		if !strings.Contains(text, "owner@example.com") {
			t.Errorf("expected owner email in permissions, got:\n%s", text)
		}
	})
}

// --- API error responses ---

func TestDriveMockAPIError(t *testing.T) {
	t.Run("search_files_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/drive/v3/files": {code: 403, body: `{"error": {"code": 403, "message": "Insufficient Permission", "status": "PERMISSION_DENIED"}}`},
		})
		handler := handleSearchDriveFiles(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"query":             "test",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Drive API error") {
			t.Errorf("expected Drive API error, got:\n%s", text)
		}
	})

	t.Run("get_file_content_not_found", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/drive/v3/files/nonexistent": {code: 404, body: `{"error": {"code": 404, "message": "File not found"}}`},
		})
		handler := handleGetDriveFileContent(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_id":           "nonexistent",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Drive API error") {
			t.Errorf("expected Drive API error, got:\n%s", text)
		}
	})

	t.Run("create_file_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/upload/drive/v3/files": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := handleCreateDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_name":         "fail.txt",
			"content":           "data",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Drive API error") {
			t.Errorf("expected Drive API error, got:\n%s", text)
		}
	})

	t.Run("share_file_permission_denied", func(t *testing.T) {
		ts := driveFakeServer(t, map[string]any{
			"/drive/v3/files/file001/permissions": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error": {"code": 403, "message": "Insufficient Permission"}}`)
			},
			"/drive/v3/files/file001": func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"file001","name":"Doc.docx","mimeType":"text/plain","webViewLink":"https://drive.google.com/file/d/file001/view"}`)
			},
		})
		handler := handleShareDriveFile(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"file_id":           "file001",
			"share_with":        "user@example.com",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Drive API error") {
			t.Errorf("expected Drive API error, got:\n%s", text)
		}
	})
}
