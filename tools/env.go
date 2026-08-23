package tools

// Environment parameters this server accepts. host-manifest and error text
// must stay aligned with these names.
const (
	EnvUserGoogleEmail = "USER_GOOGLE_EMAIL"
	EnvPSEAPIKey       = "GOOGLE_PSE_API_KEY" //nolint:gosec // G101: env var name, not a credential
	EnvPSEEngineID     = "GOOGLE_PSE_ENGINE_ID"
)
