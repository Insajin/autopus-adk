package config

import "fmt"

// Capture modes for frontend UX verification.
//
// `screenshot` is the historical behaviour: pixels are the primary UX oracle.
// `no-capture` exists because a screenshot is not always available or allowed -
// a headless CI box with no image channel, a project that refuses to store
// pixels of production data, a reviewer reading a text transcript - and a run
// without pixels must still answer the questions a screenshot answered instead
// of silently degrading to "no findings".
const (
	VerifyCaptureScreenshot = "screenshot"
	VerifyCaptureNoCapture  = "no-capture"
)

// VerifyConf is the frontend UX verification configuration.
type VerifyConf struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultViewport string `yaml:"default_viewport"`
	AutoFix         bool   `yaml:"auto_fix"`
	MaxFixAttempts  int    `yaml:"max_fix_attempts"`
	Capture         string `yaml:"capture,omitempty"`
}

// CaptureMode resolves the declared capture mode. An empty value means the
// project never thought about it, which keeps the screenshot oracle.
func (v VerifyConf) CaptureMode() string {
	if v.Capture == "" {
		return VerifyCaptureScreenshot
	}
	return v.Capture
}

// IsNoCapture reports whether verification must run without screenshots.
func (v VerifyConf) IsNoCapture() bool {
	return v.CaptureMode() == VerifyCaptureNoCapture
}

// Validate rejects any capture mode outside the two supported values. A typo
// used to be indistinguishable from the default, which is the failure mode this
// setting exists to prevent: a run that quietly took screenshots nobody wanted,
// or skipped the semantic oracles nobody noticed were missing.
func (v VerifyConf) Validate() error {
	switch v.Capture {
	case "", VerifyCaptureScreenshot, VerifyCaptureNoCapture:
		return nil
	default:
		return fmt.Errorf("verify: capture %q must be %q or %q",
			v.Capture, VerifyCaptureScreenshot, VerifyCaptureNoCapture)
	}
}
