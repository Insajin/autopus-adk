package controlplane

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/insajin/autopus-adk/pkg/worker/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetUnsignedWarnOnce re-arms the once guard for test isolation so each test
// can observe the single warning independently. Not for production use.
func resetUnsignedWarnOnce() {
	unsignedWarnOnce = sync.Once{}
}

// TestUnsignedControlPlane_WarnsOnceAndReturnsNil is the S5 oracle for
// REQ-005/REQ-007: when the signing secret is unset AND the operator has
// explicitly opted out via AllowUnsignedControlPlaneEnv, the verification
// entry points take the fail-open path, return nil, and emit exactly one
// warning per process that never includes the secret value.
//
// This test is intentionally NOT parallel: it captures the global log output
// and resets the package-level once guard, both of which are process-wide.
func TestUnsignedControlPlane_WarnsOnceAndReturnsNil(t *testing.T) {
	// Given: the signing secret is unset and unsigned mode is explicitly allowed.
	t.Setenv(PolicySigningSecretEnv, "")
	t.Setenv(AllowUnsignedControlPlaneEnv, "1")
	resetUnsignedWarnOnce()

	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	policy := security.SecurityPolicy{AllowNetwork: true}

	// When: ValidateSecurityPolicySignature is called twice with no signature.
	err1 := ValidateSecurityPolicySignature("task-1", policy, "")
	err2 := ValidateSecurityPolicySignature("task-1", policy, "")

	// Then: both calls return exactly nil.
	require.NoError(t, err1)
	require.NoError(t, err2)

	// And: exactly one warning naming the disabled verification is emitted.
	logged := buf.String()
	assert.Equal(t, 1, strings.Count(logged, "[controlplane]"),
		"warning must be emitted exactly once per process")
	assert.Contains(t, logged, PolicySigningSecretEnv)
	assert.Contains(t, logged, "fail-open")

	// And: SignedControlPlaneEnforced reports false and emits no extra warning.
	before := strings.Count(buf.String(), "[controlplane]")
	assert.False(t, SignedControlPlaneEnforced())
	after := strings.Count(buf.String(), "[controlplane]")
	assert.Equal(t, before, after, "SignedControlPlaneEnforced must not emit a warning")
}

// TestUnsignedControlPlane_AllEntryPointsShareOnce verifies that the three
// verification entry points share a single process-wide warning guard: across
// all of them, only one warning is emitted while the secret is unset and
// unsigned mode is explicitly allowed.
func TestUnsignedControlPlane_AllEntryPointsShareOnce(t *testing.T) {
	t.Setenv(PolicySigningSecretEnv, "")
	t.Setenv(AllowUnsignedControlPlaneEnv, "1")
	resetUnsignedWarnOnce()

	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	require.NoError(t, ValidateSecurityPolicySignature("t", security.SecurityPolicy{}, ""))
	require.NoError(t, VerifyCachedPolicyFile("autopus-policy-t.json", security.SecurityPolicy{}))
	require.NoError(t, ValidateControlPlaneSignature("t", "", nil, nil, nil, nil, nil, ""))

	assert.Equal(t, 1, strings.Count(buf.String(), "[controlplane]"),
		"all unsigned entry points must share one process-wide warning")
}
