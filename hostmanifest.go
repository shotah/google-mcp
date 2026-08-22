package main

import (
	"encoding/json"
	"os"
)

func writeHostManifest() error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"name":      "google",
		"command":   "google-mcp",
		"args":      []string{"--preset", "everyday"},
		"auth_args": []string{"auth"},
		"auth_flow": "pkce",
		"env_keys":  []string{"GOOGLE_OAUTH_CLIENT_ID", "GOOGLE_OAUTH_CLIENT_SECRET"},
		"blurb":     "Workspace. Client id/secret, then OAuth hop.",
	})
}
