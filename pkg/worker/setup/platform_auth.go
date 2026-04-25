package setup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// apiResponse is the standard backend response wrapper: { success, data, error }.
type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

// DeviceCode holds the response from the device authorization endpoint.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse holds the OAuth token returned after successful authorization.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// RequestDeviceCode initiates the platform device authorization flow.
func RequestDeviceCode(backendURL, codeVerifier string) (*DeviceCode, error) {
	endpoint := strings.TrimRight(backendURL, "/") + "/api/v1/auth/device/code"
	_, challenge, err := deriveChallengeFromVerifier(codeVerifier)
	if err != nil {
		return nil, err
	}

	payload := map[string]string{
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, respBody)
	}

	dc, err := unwrap[DeviceCode](respBody)
	if err != nil {
		return nil, fmt.Errorf("decode device code response: %w", err)
	}
	return dc, nil
}

// PollForToken polls the token endpoint until the user authorizes or the context is cancelled.
func PollForToken(ctx context.Context, backendURL, deviceCode, codeVerifier string, interval int) (*TokenResponse, error) {
	endpoint := strings.TrimRight(backendURL, "/") + "/api/v1/auth/device/token"
	if interval <= 0 {
		interval = 5
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}

		token, status, err := tryTokenExchange(ctx, endpoint, deviceCode, codeVerifier)
		if err != nil {
			return nil, err
		}
		switch status {
		case pollPending:
			continue
		case pollSlowDown:
			interval += 5
			continue
		case pollDone:
			return token, nil
		}
	}
}

// RefreshToken exchanges a refresh token for a new access token via the backend.
func RefreshToken(ctx context.Context, backendURL, refreshToken string) (*TokenResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(backendURL, "/")+"/api/v1/auth/cli-refresh",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, respBody)
	}

	token, err := unwrap[TokenResponse](respBody)
	if err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	return token, nil
}

func unwrap[T any](body []byte) (*T, error) {
	var wrapper apiResponse
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Data != nil {
		var result T
		if err := json.Unmarshal(wrapper.Data, &result); err != nil {
			return nil, fmt.Errorf("decode data field: %w", err)
		}
		return &result, nil
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func deriveChallengeFromVerifier(verifier string) (string, string, error) {
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

type pollStatus int

const (
	pollDone pollStatus = iota
	pollPending
	pollSlowDown
)

func tryTokenExchange(ctx context.Context, endpoint, deviceCode, codeVerifier string) (*TokenResponse, pollStatus, error) {
	payload := map[string]string{
		"device_code":   deviceCode,
		"code_verifier": codeVerifier,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, pollDone, fmt.Errorf("marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, pollDone, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, pollDone, ctx.Err()
		}
		return nil, pollDone, fmt.Errorf("poll token: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusTooManyRequests {
		switch code := extractErrorCode(respBody); code {
		case "authorization_pending":
			return nil, pollPending, nil
		case "slow_down":
			return nil, pollSlowDown, nil
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, pollDone, fmt.Errorf("token request failed (%d): %s", resp.StatusCode, respBody)
	}

	token, err := unwrap[TokenResponse](respBody)
	if err != nil {
		return nil, pollDone, fmt.Errorf("decode token response: %w", err)
	}
	return token, pollDone, nil
}

func extractErrorCode(body []byte) string {
	var plain struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &plain) == nil && plain.Error != "" {
		return plain.Error
	}

	var wrapped struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &wrapped) == nil && wrapped.Error.Code != "" {
		return wrapped.Error.Code
	}
	return ""
}
