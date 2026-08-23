package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/shotah/google-mcp/tools"
)

func writeHostManifest() error {
	return encodeHostManifest(os.Stdout)
}

func encodeHostManifest(w io.Writer) error {
	return json.NewEncoder(w).Encode(map[string]any{
		"name":      "google",
		"command":   "google-mcp",
		"args":      []string{"--preset", "everyday"},
		"auth_args": []string{"auth"},
		"auth_flow": "pkce",
		"env_keys":  []string{"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET"},
		"optional_env_keys": []string{
			tools.EnvUserGoogleEmail,
			tools.EnvPSEAPIKey,
			tools.EnvPSEEngineID,
		},
		"blurb": "Workspace. Client id/secret, then OAuth hop. Optional: USER_GOOGLE_EMAIL. PSE search needs GOOGLE_PSE_API_KEY + GOOGLE_PSE_ENGINE_ID.",
	})
}
