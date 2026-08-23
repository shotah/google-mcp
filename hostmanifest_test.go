package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/shotah/google-mcp/tools"
)

func TestHostManifestListsOptionalEnvParams(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := encodeHostManifest(&buf); err != nil {
		t.Fatalf("encodeHostManifest() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v\n%s", err, buf.String())
	}

	optional, _ := got["optional_env_keys"].([]any)
	listed := map[string]bool{}
	for _, k := range optional {
		listed[k.(string)] = true
	}
	for _, want := range []string{tools.EnvUserGoogleEmail, tools.EnvPSEAPIKey, tools.EnvPSEEngineID} {
		if !listed[want] {
			t.Errorf("optional_env_keys missing %q: %v", want, optional)
		}
	}

	required, _ := got["env_keys"].([]any)
	for _, k := range required {
		if k.(string) == tools.EnvUserGoogleEmail {
			t.Fatal("USER_GOOGLE_EMAIL must be optional, not required")
		}
	}
}
