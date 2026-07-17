package controlplane

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnforceSignedControlPlane_SecretUnsetOptOutUnset is the S4 oracle for
// REQ-004/REQ-007: startup enforcement fails closed when neither the signing
// secret nor the unsigned opt-out are set, and the error names the
// signing-secret env var without leaking any secret value.
func TestEnforceSignedControlPlane_SecretUnsetOptOutUnset(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "")
	t.Setenv(AllowUnsignedControlPlaneEnv, "")

	err := EnforceSignedControlPlane()

	require.Error(t, err)
	assert.Contains(t, err.Error(), PolicySigningSecretEnv)
}

// TestEnforceSignedControlPlane_OptOutAllowsStartup covers the REQ-005
// opt-out branch: startup enforcement passes when the operator explicitly
// allows unsigned mode.
func TestEnforceSignedControlPlane_OptOutAllowsStartup(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "")
	t.Setenv(AllowUnsignedControlPlaneEnv, "1")
	resetUnsignedWarnOnce()

	assert.NoError(t, EnforceSignedControlPlane())
}

// TestEnforceSignedControlPlane_SecretSetAllowsStartup covers the signed
// branch: startup enforcement passes once the signing secret is configured,
// regardless of the opt-out setting.
func TestEnforceSignedControlPlane_SecretSetAllowsStartup(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "secret")
	t.Setenv(AllowUnsignedControlPlaneEnv, "")

	assert.NoError(t, EnforceSignedControlPlane())
}

// TestUnsignedControlPlaneAllowed_ParsesTruthyValues verifies the opt-out
// parser accepts strconv.ParseBool truthy forms and defaults to false when
// unset or non-boolean.
func TestUnsignedControlPlaneAllowed_ParsesTruthyValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"unset", "", false},
		{"one", "1", true},
		{"true", "true", true},
		{"false", "false", false},
		{"garbage", "yesplease", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(AllowUnsignedControlPlaneEnv, tc.value)
			assert.Equal(t, tc.want, UnsignedControlPlaneAllowed())
		})
	}
}

// TestRequestIntakeFailsClosed_SecretUnsetOptOutUnset is the S6 oracle for
// REQ-006: the three request-intake verification entry points return
// non-nil errors instead of fail-open nil when neither the signing secret
// nor the unsigned opt-out are set.
func TestRequestIntakeFailsClosed_SecretUnsetOptOutUnset(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "")
	t.Setenv(AllowUnsignedControlPlaneEnv, "")

	policy := security.SecurityPolicy{AllowNetwork: true}

	err1 := ValidateSecurityPolicySignature("task-1", policy, "")
	err2 := ValidateControlPlaneSignature("task-1", "", nil, nil, nil, nil, nil, "")
	err3 := VerifyCachedPolicyFile("autopus-policy-task-1.json", policy)

	assert.Error(t, err1, "ValidateSecurityPolicySignature must fail closed")
	assert.Error(t, err2, "ValidateControlPlaneSignature must fail closed")
	assert.Error(t, err3, "VerifyCachedPolicyFile must fail closed")
}
