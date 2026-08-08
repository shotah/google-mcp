// Package auth handles OAuth2 authentication for Google APIs.
package auth

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// DefaultScopes contains the OAuth2 scopes required by the MCP server.
// userinfo.email / openid are required so post-auth email resolution works
// (chat /auth and interactive auth both call the userinfo endpoint).
var DefaultScopes = []string{
	"openid",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/documents",
	"https://www.googleapis.com/auth/spreadsheets",
	"https://www.googleapis.com/auth/presentations",
	"https://www.googleapis.com/auth/tasks",
	"https://www.googleapis.com/auth/contacts",
	"https://www.googleapis.com/auth/chat.spaces",
	"https://www.googleapis.com/auth/forms",
	"https://www.googleapis.com/auth/script.projects",
}

// NewOAuthConfig creates an OAuth2 config for Google APIs.
func NewOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       DefaultScopes,
		RedirectURL:  "http://localhost:4100/code",
	}
}
