package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyConf_ValidateCaptureMode pins the accepted vocabulary. A typo used
// to be indistinguishable from the default, so it silently kept screenshots on.
func TestVerifyConf_ValidateCaptureMode(t *testing.T) {
	t.Parallel()

	for _, accepted := range []string{"", VerifyCaptureScreenshot, VerifyCaptureNoCapture} {
		require.NoError(t, VerifyConf{Capture: accepted}.Validate(), "capture %q", accepted)
	}

	for _, rejected := range []string{"pixels", "Screenshot", "no_capture", "nocapture", " "} {
		err := VerifyConf{Capture: rejected}.Validate()
		require.Error(t, err, "capture %q", rejected)
		assert.Contains(t, err.Error(), `must be "screenshot" or "no-capture"`)
	}

	assert.Equal(t,
		`verify: capture "pixels" must be "screenshot" or "no-capture"`,
		VerifyConf{Capture: "pixels"}.Validate().Error())
}

// TestVerifyConf_CaptureModeDefaultsToScreenshot keeps every pre-existing config
// on pixels: the setting is opt-in, not a silent behaviour change.
func TestVerifyConf_CaptureModeDefaultsToScreenshot(t *testing.T) {
	t.Parallel()

	assert.Equal(t, VerifyCaptureScreenshot, VerifyConf{}.CaptureMode())
	assert.False(t, VerifyConf{}.IsNoCapture())
	assert.True(t, VerifyConf{Capture: VerifyCaptureNoCapture}.IsNoCapture())
	assert.Equal(t, VerifyCaptureScreenshot, DefaultFullConfig("test").Verify.CaptureMode())
}

// TestHarnessConfig_ValidateRejectsUnknownCaptureMode proves the field is
// reachable from the top-level gate, not only from its own method.
func TestHarnessConfig_ValidateRejectsUnknownCaptureMode(t *testing.T) {
	t.Parallel()

	cfg := DefaultFullConfig("capture-project")
	require.NoError(t, cfg.Validate())

	cfg.Verify.Capture = "pixels"
	err := cfg.Validate()
	require.Error(t, err)
	assert.Equal(t, `verify: capture "pixels" must be "screenshot" or "no-capture"`, err.Error())

	cfg.Verify.Capture = VerifyCaptureNoCapture
	require.NoError(t, cfg.Validate())
}
