package controlplane

import (
	"fmt"
	"os"
	"strconv"
)

// AllowUnsignedControlPlaneEnv opts a worker into the legacy fail-open
// behavior when the signing secret is unset. This is a dev/self-host escape
// hatch: workers connected to the backend a2a broker in production should
// set PolicySigningSecretEnv instead of relying on this opt-out.
const AllowUnsignedControlPlaneEnv = "AUTOPUS_A2A_ALLOW_UNSIGNED"

// UnsignedControlPlaneAllowed reports whether the operator explicitly opted
// out of signed control-plane/policy enforcement via AllowUnsignedControlPlaneEnv.
// Any value accepted by strconv.ParseBool as true counts; unset or
// non-boolean values default to false (enforced).
func UnsignedControlPlaneAllowed() bool {
	allowed, _ := strconv.ParseBool(os.Getenv(AllowUnsignedControlPlaneEnv))
	return allowed
}

// unsignedResult decides the outcome for a call site that would otherwise
// verify a signature but found the signing secret unset. context identifies
// the call site in the returned error so operators can locate which check
// failed. When the operator has explicitly opted out via
// AllowUnsignedControlPlaneEnv, this preserves the historical warn-once
// fail-open behavior. Otherwise it fails closed with an error naming the
// signing-secret env var — never the secret value itself.
func unsignedResult(context string) error {
	if UnsignedControlPlaneAllowed() {
		warnUnsignedControlPlane()
		return nil
	}
	return fmt.Errorf(
		"%s: %s is unset; set it to enable signed control-plane/policy verification, or set %s=1 to explicitly allow unsigned mode (dev/self-host only)",
		context, PolicySigningSecretEnv, AllowUnsignedControlPlaneEnv,
	)
}

// EnforceSignedControlPlane gates worker startup against connecting to the
// backend a2a broker without a way to trust its control-plane/policy
// metadata. It returns a fail-fast error unless the signing secret is set or
// unsigned mode is explicitly allowed via AllowUnsignedControlPlaneEnv.
func EnforceSignedControlPlane() error {
	if signingSecret() != "" {
		return nil
	}
	return unsignedResult("worker startup")
}
