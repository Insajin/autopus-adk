package config

// StickyCadence reports the configured SPEC-STICKYRULE-001 prompt cadence, or 0
// when the key is absent, zero, or negative.
//
// Resolving 0 to the shipped default is deliberately NOT done here. The default
// belongs to the cadence contract in pkg/rulecond, and this package cannot
// import it: pkg/rulecond depends on pkg/adapter, which depends on this package,
// so the import would close a cycle. Returning "unset" keeps one default in one
// place instead of two that drift.
//
// A non-integer scalar never reaches this function. `sticky_cadence` is a typed
// int field, so the yaml.Unmarshal in loadConfig rejects a non-integer value as
// a config load error before any caller reads the cadence (REQ-STICKYRULE-MAP-01).
func StickyCadence(cfg *HarnessConfig) int {
	if cfg == nil || cfg.Hooks.StickyCadence <= 0 {
		return 0
	}
	return cfg.Hooks.StickyCadence
}
