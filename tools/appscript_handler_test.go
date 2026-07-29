package tools

import (
	"strings"
	"testing"
)

// --- appscript_list_projects ---
// No required params beyond email; first error is auth failure.

func TestAppScriptHandlerListScriptProjectsAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_list_projects", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- appscript_get_project ---

func TestAppScriptHandlerGetScriptProjectMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_project", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerGetScriptProjectAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_project", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- appscript_get_content ---

func TestAppScriptHandlerGetScriptContentMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_content", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerGetScriptContentMissingFileName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_content", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "file_name") {
		t.Errorf("expected error mentioning 'file_name', got %q", text)
	}
}

// --- appscript_list_deployments ---

func TestAppScriptHandlerListDeploymentsMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_list_deployments", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

// --- appscript_list_processes ---
// No required params beyond email; first error is auth failure.

func TestAppScriptHandlerListScriptProcessesAuthFailure(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_list_processes", nil)
	if !isError {
		t.Fatal("expected isError=true for auth failure")
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "credentials") && !strings.Contains(lower, "authenticating") {
		t.Errorf("expected error about credentials/auth, got %q", text)
	}
}

// --- appscript_list_versions ---

func TestAppScriptHandlerListVersionsMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_list_versions", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

// --- appscript_get_version ---

func TestAppScriptHandlerGetVersionMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_version", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerGetVersionMissingVersionNumber(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_version", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "version_number") {
		t.Errorf("expected error mentioning 'version_number', got %q", text)
	}
}

// --- appscript_get_metrics ---

func TestAppScriptHandlerGetScriptMetricsMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_get_metrics", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

// --- appscript_create_project ---

func TestAppScriptHandlerCreateScriptProjectMissingTitle(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_create_project", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "title") {
		t.Errorf("expected error mentioning 'title', got %q", text)
	}
}

// --- appscript_update_content ---

func TestAppScriptHandlerUpdateScriptContentMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_update_content", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerUpdateScriptContentMissingFiles(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_update_content", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "files") {
		t.Errorf("expected error mentioning 'files', got %q", text)
	}
}

// --- appscript_run_function ---

func TestAppScriptHandlerRunScriptFunctionMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_run_function", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerRunScriptFunctionMissingFunctionName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_run_function", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "function_name") {
		t.Errorf("expected error mentioning 'function_name', got %q", text)
	}
}

// --- appscript_create_deployment ---

func TestAppScriptHandlerCreateDeploymentMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_create_deployment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerCreateDeploymentMissingDescription(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_create_deployment", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "description") {
		t.Errorf("expected error mentioning 'description', got %q", text)
	}
}

// --- appscript_update_deployment ---

func TestAppScriptHandlerUpdateDeploymentMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_update_deployment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerUpdateDeploymentMissingDeploymentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_update_deployment", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "deployment_id") {
		t.Errorf("expected error mentioning 'deployment_id', got %q", text)
	}
}

// --- appscript_delete_deployment ---

func TestAppScriptHandlerDeleteDeploymentMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_delete_deployment", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

func TestAppScriptHandlerDeleteDeploymentMissingDeploymentID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_delete_deployment", map[string]any{
		"script_id": "abc123",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "deployment_id") {
		t.Errorf("expected error mentioning 'deployment_id', got %q", text)
	}
}

// --- appscript_delete_project ---

func TestAppScriptHandlerDeleteScriptProjectMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_delete_project", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

// --- appscript_create_version ---

func TestAppScriptHandlerCreateVersionMissingScriptID(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_create_version", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "script_id") {
		t.Errorf("expected error mentioning 'script_id', got %q", text)
	}
}

// --- appscript_generate_trigger_code ---
// appscript_generate_trigger_code does NOT require auth — it generates code locally.
// It requires trigger_type and function_name.

func TestAppScriptHandlerGenerateTriggerCodeMissingTriggerType(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_generate_trigger_code", nil)
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "trigger_type") {
		t.Errorf("expected error mentioning 'trigger_type', got %q", text)
	}
}

func TestAppScriptHandlerGenerateTriggerCodeMissingFunctionName(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_generate_trigger_code", map[string]any{
		"trigger_type": "time_daily",
	})
	if !isError {
		t.Fatal("expected isError=true")
	}
	if !strings.Contains(strings.ToLower(text), "function_name") {
		t.Errorf("expected error mentioning 'function_name', got %q", text)
	}
}

func TestAppScriptHandlerGenerateTriggerCodeSuccess(t *testing.T) {
	s := newToolTestServer(t)
	text, isError := callTool(t, s, "appscript_generate_trigger_code", map[string]any{
		"trigger_type":  "time_daily",
		"function_name": "myFunction",
	})
	if isError {
		t.Fatalf("expected success, got error: %s", text)
	}
	// Should contain generated code
	if !strings.Contains(text, "myFunction") {
		t.Errorf("expected output to contain 'myFunction', got %q", text)
	}
	if !strings.Contains(text, "TRIGGER") {
		t.Errorf("expected output to contain 'TRIGGER' header, got %q", text)
	}
}
