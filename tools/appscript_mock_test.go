package tools

import (
	"strings"
	"testing"
)

// --- appscript_list_projects ---

func TestAppScriptMockListScriptProjects(t *testing.T) {
	t.Run("success_with_projects", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{
				"files": [
					{"id":"script001","name":"My Script","createdTime":"2026-01-10T10:00:00Z","modifiedTime":"2026-02-15T08:00:00Z"},
					{"id":"script002","name":"Auto Mailer","createdTime":"2026-01-05T12:00:00Z","modifiedTime":"2026-02-10T14:00:00Z"}
				]
			}`,
		})
		handler := makeListScriptProjectsHandler(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Found 2 Apps Script projects") {
			t.Errorf("expected 'Found 2 Apps Script projects', got:\n%s", text)
		}
		if !strings.Contains(text, "My Script") {
			t.Errorf("expected 'My Script' in output")
		}
		if !strings.Contains(text, "Auto Mailer") {
			t.Errorf("expected 'Auto Mailer' in output")
		}
		if !strings.Contains(text, "script001") {
			t.Errorf("expected script ID in output")
		}
	})

	t.Run("success_no_projects", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/drive/v3/files": `{"files":[]}`,
		})
		handler := makeListScriptProjectsHandler(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No Apps Script projects found") {
			t.Errorf("expected 'No Apps Script projects found', got:\n%s", text)
		}
	})
}

// --- appscript_create_project ---

func TestAppScriptMockCreateScriptProject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects": map[string]any{
				"scriptId": "newscript001",
				"title":    "New Automation",
			},
		})
		handler := makeCreateScriptProjectHandler(testClientFunc(ts))
		text := callHandlerOK(t, handler, map[string]any{
			"title":             "New Automation",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Created Apps Script project") {
			t.Errorf("expected 'Created Apps Script project', got:\n%s", text)
		}
		if !strings.Contains(text, "New Automation") {
			t.Errorf("expected title in output")
		}
		if !strings.Contains(text, "newscript001") {
			t.Errorf("expected script ID in output")
		}
	})
}

// --- project content and metadata ---

func TestAppScriptMockGetScriptProject(t *testing.T) {
	ts := driveFakeServer(t, map[string]any{
		"/v1/projects/script001/content": map[string]any{
			"files": []map[string]any{
				{"name": "Code", "type": "SERVER_JS", "source": "function run() { return 'ok'; }"},
			},
		},
		"/v1/projects/script001": map[string]any{
			"scriptId":   "script001",
			"title":      "Daily Report",
			"creator":    map[string]any{"email": "owner@example.com"},
			"createTime": "2026-01-10T10:00:00Z",
			"updateTime": "2026-02-15T08:00:00Z",
		},
	})
	text := callHandlerOK(t, makeGetScriptProjectHandler(testClientFunc(ts)), map[string]any{
		"script_id": "script001", "user_google_email": "test@example.com",
	})
	for _, want := range []string{"Project: Daily Report", "owner@example.com", "Files:", "Code (SERVER_JS)", "function run()"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestAppScriptMockGetScriptContent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/content": `{"files":[{"name":"Code","type":"SERVER_JS","source":"function hello() {}"}]}`,
		})
		text := callHandlerOK(t, makeGetScriptContentHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "file_name": "Code", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "File: Code (SERVER_JS)\n\nfunction hello()") {
			t.Errorf("expected file content, got:\n%s", text)
		}
	})
	t.Run("not_found", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/content": `{"files":[{"name":"Code","type":"SERVER_JS"}]}`,
		})
		text := callHandlerOK(t, makeGetScriptContentHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "file_name": "Missing", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "File 'Missing' not found in project script001") {
			t.Errorf("expected not-found response, got:\n%s", text)
		}
	})
}

// --- deployments, processes, and versions ---

func TestAppScriptMockListDeployments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/deployments": `{"deployments":[{"deploymentId":"dep001","deploymentConfig":{"description":"Production"},"updateTime":"2026-02-15T08:00:00Z"}]}`,
		})
		text := callHandlerOK(t, makeListDeploymentsHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Production (dep001)") {
			t.Errorf("expected deployment details, got:\n%s", text)
		}
	})
	t.Run("empty", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{"/v1/projects/script001/deployments": `{"deployments":[]}`})
		text := callHandlerOK(t, makeListDeploymentsHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "No deployments found for script: script001") {
			t.Errorf("expected empty response, got:\n%s", text)
		}
	})
}

func TestAppScriptMockListScriptProcesses(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/processes": `{"processes":[{"functionName":"sendReport","processStatus":"COMPLETED","startTime":"2026-02-15T08:00:00Z","duration":"12s"}]}`,
	})
	text := callHandlerOK(t, makeListScriptProcessesHandler(testClientFunc(ts)), map[string]any{
		"script_id": "script001", "page_size": 10, "user_google_email": "test@example.com",
	})
	for _, want := range []string{"sendReport", "Status: COMPLETED", "Duration: 12s"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestAppScriptMockVersions(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/versions": `{"versions":[{"versionNumber":3,"description":"Release 3","createTime":"2026-02-15T08:00:00Z"}]}`,
		})
		text := callHandlerOK(t, makeListVersionsHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Version 3: Release 3") {
			t.Errorf("expected version details, got:\n%s", text)
		}
	})
	t.Run("get", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/versions/3": `{"versionNumber":3,"description":"Release 3","createTime":"2026-02-15T08:00:00Z"}`,
		})
		text := callHandlerOK(t, makeGetVersionHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "version_number": 3, "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Version 3 of script: script001") {
			t.Errorf("expected version details, got:\n%s", text)
		}
	})
}

func TestAppScriptMockGetScriptMetrics(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/projects/script001/metrics": `{
			"activeUsers":[{"startTime":"2026-02-01T00:00:00Z","endTime":"2026-02-02T00:00:00Z","value":"4"}],
			"totalExecutions":[{"startTime":"2026-02-01T00:00:00Z","endTime":"2026-02-02T00:00:00Z","value":"12"}],
			"failedExecutions":[{"startTime":"2026-02-01T00:00:00Z","endTime":"2026-02-02T00:00:00Z","value":"1"}]
		}`,
	})
	text := callHandlerOK(t, makeGetScriptMetricsHandler(testClientFunc(ts)), map[string]any{
		"script_id": "script001", "metrics_granularity": "WEEKLY", "user_google_email": "test@example.com",
	})
	for _, want := range []string{"Granularity: WEEKLY", "4 users", "12 executions", "1 failures"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output:\n%s", want, text)
		}
	}
}

// --- mutations ---

func TestAppScriptMockUpdateScriptContent(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/projects/script001/content": `{"files":[{"name":"Code","type":"SERVER_JS"},{"name":"appsscript","type":"JSON"}]}`,
	})
	text := callHandlerOK(t, makeUpdateScriptContentHandler(testClientFunc(ts)), map[string]any{
		"script_id": "script001",
		"files": []any{
			map[string]any{"name": "Code", "type": "SERVER_JS", "source": "function run() {}"},
			map[string]any{"name": "appsscript", "type": "JSON", "source": "{}"},
		},
		"user_google_email": "test@example.com",
	})
	for _, want := range []string{"Updated script project: script001", "Code (SERVER_JS)", "appsscript (JSON)"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in output:\n%s", want, text)
		}
	}
}

func TestAppScriptMockRunScriptFunction(t *testing.T) {
	ts := fakeAPIServer(t, map[string]any{
		"/v1/scripts/script001:run": `{"response":{"result":"completed"}}`,
	})
	text := callHandlerOK(t, makeRunScriptFunctionHandler(testClientFunc(ts)), map[string]any{
		"script_id": "script001", "function_name": "sendReport", "parameters": []any{"weekly"}, "dev_mode": true, "user_google_email": "test@example.com",
	})
	if !strings.Contains(text, "Execution successful\nFunction: sendReport\nResult: completed") {
		t.Errorf("expected execution result, got:\n%s", text)
	}
}

func TestAppScriptMockDeployments(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/versions":    `{"versionNumber":4,"description":"Release 4"}`,
			"/v1/projects/script001/deployments": `{"deploymentId":"dep004"}`,
		})
		text := callHandlerOK(t, makeCreateDeploymentHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "description": "Production", "version_description": "Release 4", "user_google_email": "test@example.com",
		})
		for _, want := range []string{"Created deployment for script: script001", "Deployment ID: dep004", "Version: 4"} {
			if !strings.Contains(text, want) {
				t.Errorf("expected %q in output:\n%s", want, text)
			}
		}
	})
	t.Run("update", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/deployments/dep001": `{"deploymentId":"dep001","deploymentConfig":{"description":"Updated production"}}`,
		})
		text := callHandlerOK(t, makeUpdateDeploymentHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "deployment_id": "dep001", "description": "Updated production", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Description: Updated production") {
			t.Errorf("expected update response, got:\n%s", text)
		}
	})
	t.Run("delete", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{"/v1/projects/script001/deployments/dep001": `{}`})
		text := callHandlerOK(t, makeDeleteDeploymentHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "deployment_id": "dep001", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Deleted deployment: dep001 from script: script001") {
			t.Errorf("expected delete response, got:\n%s", text)
		}
	})
}

func TestAppScriptMockCreateVersionAndDeleteProject(t *testing.T) {
	t.Run("appscript_create_version", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{
			"/v1/projects/script001/versions": `{"versionNumber":5,"description":"Release 5","createTime":"2026-02-15T08:00:00Z"}`,
		})
		text := callHandlerOK(t, makeCreateVersionHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "description": "Release 5", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Created version 5 for script: script001") {
			t.Errorf("expected create version response, got:\n%s", text)
		}
	})
	t.Run("delete_project", func(t *testing.T) {
		ts := fakeAPIServer(t, map[string]any{"/drive/v3/files/script001": 200})
		text := callHandlerOK(t, makeDeleteScriptProjectHandler(testClientFunc(ts)), map[string]any{
			"script_id": "script001", "user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "Deleted Apps Script project: script001") {
			t.Errorf("expected delete project response, got:\n%s", text)
		}
	})
}

// --- appscript_generate_trigger_code (pure code generation, no API) ---

func TestAppScriptMockGenerateTriggerCode(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// This handler doesn't need a fake server — it generates code locally.
		s := newToolTestServer(t)
		text, isError := callTool(t, s, "appscript_generate_trigger_code", map[string]any{
			"trigger_type":  "time_daily",
			"function_name": "sendDailyReport",
		})
		if isError {
			t.Fatalf("expected success, got error: %s", text)
		}
		if !strings.Contains(text, "sendDailyReport") {
			t.Errorf("expected function name in output, got:\n%s", text)
		}
		if !strings.Contains(text, "function") {
			t.Errorf("expected 'function' keyword in generated code, got:\n%s", text)
		}
	})
}

// --- API error responses ---

func TestAppScriptMockAPIError(t *testing.T) {
	t.Run("list_projects_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/drive/v3/files": {code: 403, body: `{"error": {"code": 403, "message": "Forbidden"}}`},
		})
		handler := makeListScriptProjectsHandler(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "listing script projects") {
			t.Errorf("expected listing error, got:\n%s", text)
		}
	})

	t.Run("create_project_error", func(t *testing.T) {
		ts := fakeAPIServerWithStatus(t, map[string]statusResponse{
			"/v1/projects": {code: 500, body: `{"error": {"code": 500, "message": "Internal Server Error"}}`},
		})
		handler := makeCreateScriptProjectHandler(testClientFunc(ts))
		text := callHandlerErr(t, handler, map[string]any{
			"title":             "Bad Script",
			"user_google_email": "test@example.com",
		})
		if !strings.Contains(text, "creating script project") {
			t.Errorf("expected creation error, got:\n%s", text)
		}
	})
}
