package config

// UsageProfile represents how the user intends to use ADK.
type UsageProfile string

const (
	ProfileDeveloper UsageProfile = "developer"
	ProfileFullstack UsageProfile = "fullstack"
)

// DefaultUsageProfile returns the default profile for backward compatibility.
func DefaultUsageProfile() UsageProfile {
	return ProfileDeveloper
}

// IsValid checks if the profile value is recognized.
func (p UsageProfile) IsValid() bool {
	return p == ProfileDeveloper || p == ProfileFullstack || p == ""
}

// Effective returns the effective profile, defaulting empty to developer.
func (p UsageProfile) Effective() UsageProfile {
	if p == "" {
		return ProfileDeveloper
	}
	return p
}

// HintsConf holds configuration for non-intrusive platform hints.
type HintsConf struct {
	// Platform controls whether platform upgrade hints are shown.
	// nil (omitted) = enabled (default), explicit false = disabled permanently.
	Platform *bool `yaml:"platform,omitempty"`
}

// IsPlatformHintEnabled returns whether platform hints are active.
func (h HintsConf) IsPlatformHintEnabled() bool {
	if h.Platform == nil {
		return true
	}
	return *h.Platform
}
