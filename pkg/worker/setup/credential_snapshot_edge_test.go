package setup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthTokenFromCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		creds *rawCredentials
		want  string
	}{
		{name: "nil credentials", creds: nil, want: ""},
		{
			name:  "api key auth type returns api key",
			creds: &rawCredentials{AuthType: "api_key", APIKey: "acos_key"},
			want:  "acos_key",
		},
		{
			name:  "access token preferred when no api key auth",
			creds: &rawCredentials{AccessToken: "jwt-token"},
			want:  "jwt-token",
		},
		{
			name:  "api key auth without api key falls through to access token",
			creds: &rawCredentials{AuthType: "api_key", AccessToken: "jwt-fallback"},
			want:  "jwt-fallback",
		},
		{
			name:  "no usable token",
			creds: &rawCredentials{AuthType: "jwt"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, authTokenFromCredentials(tt.creds))
		})
	}
}

func TestAuthStateFromCredentials(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)

	tests := []struct {
		name      string
		creds     *rawCredentials
		wantValid bool
		wantType  string
	}{
		{name: "nil credentials", creds: nil, wantValid: false, wantType: "none"},
		{
			name:      "api key always valid",
			creds:     &rawCredentials{APIKey: "acos_key"},
			wantValid: true,
			wantType:  "api_key",
		},
		{
			name:      "jwt without expiry is valid",
			creds:     &rawCredentials{AccessToken: "jwt"},
			wantValid: true,
			wantType:  "jwt",
		},
		{
			name:      "jwt with future expiry is valid",
			creds:     &rawCredentials{AccessToken: "jwt", ExpiresAt: future},
			wantValid: true,
			wantType:  "jwt",
		},
		{
			name:      "jwt with past expiry is invalid",
			creds:     &rawCredentials{AccessToken: "jwt", ExpiresAt: past},
			wantValid: false,
			wantType:  "jwt",
		},
		{
			name:      "jwt with unparseable expiry treated as valid",
			creds:     &rawCredentials{AccessToken: "jwt", ExpiresAt: "not-a-date"},
			wantValid: true,
			wantType:  "jwt",
		},
		{
			name:      "empty credentials",
			creds:     &rawCredentials{},
			wantValid: false,
			wantType:  "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			valid, typ := authStateFromCredentials(tt.creds)
			assert.Equal(t, tt.wantValid, valid)
			assert.Equal(t, tt.wantType, typ)
		})
	}
}
