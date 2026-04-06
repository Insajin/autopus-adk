// Package setup - apikey.go: Worker API Key credential persistence.
//
// Workers may authenticate using a long-lived Worker API Key instead of the
// PKCE device-code flow. The key is stored in credentials.json with
// auth_type="api_key" so CollectStatus can distinguish it from JWT tokens.
package setup

import "time"

// SaveAPIKeyCredentials persists a Worker API Key to credentials.json.
// The key is stored in plaintext — it is treated like a long-lived password.
func SaveAPIKeyCredentials(apiKey, backendURL string) error {
	return SaveCredentials(map[string]any{
		"auth_type":   "api_key",
		"api_key":     apiKey,
		"backend_url": backendURL,
		"created_at":  time.Now().Format(time.RFC3339),
	})
}

// LoadAPIKey reads the stored Worker API Key from credentials.json.
// Returns ("", nil) if no API key is configured.
func LoadAPIKey() (string, error) {
	creds, err := loadRawCredentials()
	if err != nil {
		return "", nil // missing file → no key configured
	}
	if creds.AuthType != "api_key" || creds.APIKey == "" {
		return "", nil
	}
	return creds.APIKey, nil
}

// LoadAuthToken returns the bearer token for A2A WebSocket authentication.
// It reads credentials.json and returns the appropriate token based on auth_type:
//   - auth_type="api_key" → returns the api_key field (Worker API Key)
//   - otherwise → returns the access_token field (JWT)
//
// Returns ("", nil) if no credentials are configured.
func LoadAuthToken() (string, error) {
	creds, err := loadRawCredentials()
	if err != nil {
		return "", nil
	}
	if creds.AuthType == "api_key" && creds.APIKey != "" {
		return creds.APIKey, nil
	}
	if creds.AccessToken != "" {
		return creds.AccessToken, nil
	}
	return "", nil
}
