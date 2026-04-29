package config

// VerifyConf is the frontend UX verification configuration.
type VerifyConf struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultViewport string `yaml:"default_viewport"`
	AutoFix         bool   `yaml:"auto_fix"`
	MaxFixAttempts  int    `yaml:"max_fix_attempts"`
}
