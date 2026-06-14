package journey

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/mobile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMaestroScriptedRequiresProjectLocalYAMLFlow(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "login.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobilePack("maestro-scripted")
	pack.Mobile.FlowPath = ".autopus/qa/mobile/flows/login.yaml"
	pack.Mobile.DeviceTarget = "device-ref:ios-sim"
	pack.Command = Command{Argv: []string{"maestro", "test", ".autopus/qa/mobile/flows/login.yaml"}, CWD: ".", Timeout: "60s"}

	assert.NoError(t, Validate(pack, dir))

	pack.Mobile.FlowPath = ".codex/mobile/login.yaml"
	err := Validate(pack, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project-local")
	assertValidationCode(t, err, mobile.ReasonProjectLocalFlowRequired)
}

func TestValidateMaestroScriptedRejectsCommandFlowMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "login.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobilePack("maestro-scripted")
	pack.Mobile.FlowPath = ".autopus/qa/mobile/flows/login.yaml"
	pack.Mobile.DeviceTarget = "device-ref:ios-sim"
	pack.Command = Command{Argv: []string{"maestro", "test", ".autopus/qa/mobile/flows/settings.yaml"}, CWD: ".", Timeout: "60s"}

	err := Validate(pack, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow")
}

func TestValidateMaestroScriptedRejectsUnsafeMobileCommandFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "login.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobilePack("maestro-scripted")
	pack.Mobile.FlowPath = ".autopus/qa/mobile/flows/login.yaml"
	pack.Mobile.DeviceTarget = "device-ref:ios-sim"
	pack.Command = Command{Argv: []string{"maestro", "test", "--config", "/tmp/maestro.yaml", ".autopus/qa/mobile/flows/login.yaml"}, CWD: ".", Timeout: "60s"}

	err := Validate(pack, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow")
}

func TestValidateAppiumMobileExploreRequiresBoundedPolicy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pack := mobilePack("appium-mobile-explore")
	pack.Command = Command{Argv: []string{"appium"}, CWD: ".", Timeout: "120s"}
	pack.Mobile = MobilePolicy{
		DeviceTarget:      "device-ref:ios-sim",
		AppArtifactDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ForbiddenActions:  []string{"payment", "email_send"},
	}

	assert.NoError(t, Validate(pack, dir))

	pack.PassFailAuthority = "ai"
	err := Validate(pack, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI pass/fail")
}

func TestValidateAppiumMobileExploreRejectsRawDeviceTargetAndFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pack := mobilePack("appium-mobile-explore")
	pack.Command = Command{Argv: []string{"appium", "--log", "/tmp/appium.log"}, CWD: ".", Timeout: "120s"}
	pack.Mobile = MobilePolicy{
		DeviceTarget:      "emulator-5554",
		AppArtifactDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ForbiddenActions:  []string{"payment", "email_send"},
	}

	err := Validate(pack, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "device target")

	pack.Mobile.DeviceTarget = "device-ref:ios-sim"
	err = Validate(pack, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "appium")
}

func TestValidateMaestroScriptedRejectsAIPassFailAuthority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "login.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	pack := mobilePack("maestro-scripted")
	pack.Mobile.FlowPath = ".autopus/qa/mobile/flows/login.yaml"
	pack.Mobile.DeviceTarget = "device-ref:ios-sim"
	pack.Command = Command{Argv: []string{"maestro", "test", ".autopus/qa/mobile/flows/login.yaml"}, CWD: ".", Timeout: "60s"}

	assert.NoError(t, Validate(pack, dir))

	pack.PassFailAuthority = "ai"
	err := Validate(pack, dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI pass/fail")
	assertValidationCode(t, err, mobilePolicyInvalidCode)
}

func mobilePack(adapterID string) Pack {
	return Pack{
		ID:      "mobile",
		Surface: "mobile",
		Lanes:   []string{"mobile-readiness"},
		Adapter: AdapterRef{ID: adapterID},
		Checks:  []Check{{ID: "deterministic", Type: "mobile_check"}},
	}
}

func assertValidationCode(t *testing.T, err error, code string) {
	t.Helper()
	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr), "expected ValidationError, got %T", err)
	assert.Equal(t, code, validationErr.Code)
}
