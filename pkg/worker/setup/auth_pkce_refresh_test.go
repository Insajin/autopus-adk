package setup

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCE_VerifierLength(t *testing.T) {
	t.Parallel()

	verifier, _, err := GeneratePKCE()
	require.NoError(t, err)
	assert.Len(t, verifier, 43)
}

func TestGeneratePKCE_VerifierIsBase64URL(t *testing.T) {
	t.Parallel()

	verifier, _, err := GeneratePKCE()
	require.NoError(t, err)

	decoded, err := base64.RawURLEncoding.DecodeString(verifier)
	require.NoError(t, err)
	assert.Len(t, decoded, 32)
}

func TestGeneratePKCE_ChallengeIsSHA256OfVerifier(t *testing.T) {
	t.Parallel()

	verifier, challenge, err := GeneratePKCE()
	require.NoError(t, err)

	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	assert.Equal(t, expected, challenge)
}

func TestGeneratePKCE_UniquePairs(t *testing.T) {
	t.Parallel()

	v1, c1, err := GeneratePKCE()
	require.NoError(t, err)

	v2, c2, err := GeneratePKCE()
	require.NoError(t, err)

	assert.NotEqual(t, v1, v2, "verifiers should be unique")
	assert.NotEqual(t, c1, c2, "challenges should be unique")
}

func TestGeneratePKCE_ChallengeLength(t *testing.T) {
	t.Parallel()

	_, challenge, err := GeneratePKCE()
	require.NoError(t, err)
	assert.Len(t, challenge, 43)
}

func TestDeriveChallengeFromVerifier(t *testing.T) {
	t.Parallel()

	verifier, originalChallenge, err := GeneratePKCE()
	require.NoError(t, err)

	_, derivedChallenge, err := deriveChallengeFromVerifier(verifier)
	require.NoError(t, err)

	assert.Equal(t, originalChallenge, derivedChallenge)
}

func TestRefreshToken_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/auth/cli-refresh", r.URL.Path)

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "old-refresh-token", body["refresh_token"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
	}))
	defer srv.Close()

	token, err := RefreshToken(context.Background(), srv.URL, "old-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", token.AccessToken)
	assert.Equal(t, "new-refresh-token", token.RefreshToken)
	assert.Equal(t, 3600, token.ExpiresIn)
}

func TestRefreshToken_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := RefreshToken(context.Background(), srv.URL, "bad-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestRefreshToken_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{invalid-json"))
	}))
	defer srv.Close()

	_, err := RefreshToken(context.Background(), srv.URL, "some-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestRefreshToken_WrappedResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"wrapped-tok","refresh_token":"ref","expires_in":3600,"token_type":"Bearer"}}`))
	}))
	defer srv.Close()

	token, err := RefreshToken(context.Background(), srv.URL, "old-token")
	require.NoError(t, err)
	assert.Equal(t, "wrapped-tok", token.AccessToken)
}

func TestRefreshToken_ConnectionRefused(t *testing.T) {
	t.Parallel()

	_, err := RefreshToken(context.Background(), "http://127.0.0.1:1", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh request")
}
