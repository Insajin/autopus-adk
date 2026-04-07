package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeviceCodeResponse holds the server's response to a device code request.
// The server proxies device code creation for OpenAI OAuth on behalf of the CLI.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"` // polling interval in seconds
}

// DeviceTokenResponse is the result of polling the device-token endpoint.
type DeviceTokenResponse struct {
	Status       string `json:"status"`                  // "pending", "completed", "expired"
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

// deviceCodePayload is the request body for device-code creation.
type deviceCodePayload struct {
	Provider string `json:"provider"`
}

// deviceTokenPayload is the request body for device-token polling.
type deviceTokenPayload struct {
	DeviceCode string `json:"device_code"`
}

// RequestOpenAIDeviceCode calls the server endpoint to get a device code for OpenAI OAuth.
// POST {serverURL}/api/v1/workspaces/{workspaceID}/ai-oauth/device-code
func RequestOpenAIDeviceCode(ctx context.Context, serverURL, authToken, workspaceID string) (*DeviceCodeResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/ai-oauth/device-code",
		strings.TrimRight(serverURL, "/"), workspaceID)

	payload := deviceCodePayload{Provider: "openai"}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal device code request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server does not support device code flow: %d %s", resp.StatusCode, respBody)
	}

	result, err := unwrapJSON[DeviceCodeResponse](respBody)
	if err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	return result, nil
}

// PollOpenAIDeviceToken polls the server until OpenAI OAuth completes or the context is cancelled.
// POST {serverURL}/api/v1/workspaces/{workspaceID}/ai-oauth/device-token
func PollOpenAIDeviceToken(ctx context.Context, serverURL, authToken, workspaceID, deviceCode string, interval int) (*DeviceTokenResponse, error) {
	pollInterval := time.Duration(interval) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s/ai-oauth/device-token",
		strings.TrimRight(serverURL, "/"), workspaceID)

	payload := deviceTokenPayload{DeviceCode: deviceCode}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal token poll request: %w", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create token poll request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authToken)

		resp, err := httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("poll device token: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("device token poll failed (%d): %s", resp.StatusCode, respBody)
		}

		result, err := unwrapJSON[DeviceTokenResponse](respBody)
		if err != nil {
			return nil, fmt.Errorf("decode token poll response: %w", err)
		}

		switch result.Status {
		case "completed":
			return result, nil
		case "expired":
			return nil, fmt.Errorf("device code expired")
		case "pending":
			// Continue polling.
		default:
			return nil, fmt.Errorf("unknown device token status: %s", result.Status)
		}
	}
}
