package evalregression

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// TestEvalRegressionVerifyReasons asserts that the three machine-readable
// reason literals returned by EvalRegressionVerifyReasons match the
// unexported constants exactly. Callers (e.g., the CLI package) depend on
// these string values being stable and correct.
func TestEvalRegressionVerifyReasons(t *testing.T) {
	reasons := EvalRegressionVerifyReasons()

	want := map[string]string{
		"unsigned":    "artifact_unsigned",
		"invalid":     "signature_invalid",
		"key_unknown": "signature_key_unknown",
	}
	for key, expected := range want {
		if got := reasons[key]; got != expected {
			t.Errorf("EvalRegressionVerifyReasons()[%q] = %q; want %q", key, got, expected)
		}
	}
	if len(reasons) != 3 {
		t.Errorf("EvalRegressionVerifyReasons() returned %d entries; want exactly 3", len(reasons))
	}
}

// TestCommittedPublicKeysLoopBodyDefensiveCopy exercises the copy loop inside
// CommittedEvalRegressionPublicKeys by temporarily injecting a real ed25519
// public key into the package-level allowlist. It verifies:
//
//	(a) The returned map contains the injected key_id.
//	(b) Mutating the returned key bytes does NOT affect the package allowlist
//	    (deep copy — each key slice is cloned independently).
//
// The allowlist is restored via t.Cleanup so no state leaks to other tests.
func TestCommittedPublicKeysLoopBodyDefensiveCopy(t *testing.T) {
	const testKeyID = "test-loop-key"

	// Generate a real ed25519 public key to inject into the allowlist.
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Save and restore the committed allowlist around the test.
	original := evalRegressionPublicKeys
	evalRegressionPublicKeys = map[string]ed25519.PublicKey{testKeyID: pub}
	t.Cleanup(func() { evalRegressionPublicKeys = original })

	// (a) The returned map must contain the injected key with correct bytes.
	got := CommittedEvalRegressionPublicKeys()
	returnedKey, present := got[testKeyID]
	if !present {
		t.Fatalf("loop body: key_id %q missing from CommittedEvalRegressionPublicKeys()", testKeyID)
	}
	if string(returnedKey) != string(pub) {
		t.Fatalf("loop body: returned key bytes differ from injected key")
	}

	// (b) Overwrite every byte of the returned slice; the allowlist must be unchanged.
	for i := range returnedKey {
		returnedKey[i] = 0xff
	}
	if string(evalRegressionPublicKeys[testKeyID]) != string(pub) {
		t.Fatalf("deep copy violated: mutation of returned key bytes leaked into the allowlist")
	}

	// A second call must still return the original (unmodified) bytes.
	again := CommittedEvalRegressionPublicKeys()
	if string(again[testKeyID]) != string(pub) {
		t.Fatalf("deep copy violated: second call returned mutated bytes")
	}
}
