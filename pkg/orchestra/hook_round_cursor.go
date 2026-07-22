package orchestra

import "fmt"

func hookRoundCursorName(provider string) string {
	return sanitizeProviderName(providerArtifactIdentity(provider)) + "-round-cursor"
}

// WriteRoundCursor persists the logical round inherited by the next hook
// invocation. Provider CLIs keep their launch-time environment, so round 2+
// cannot rely on AUTOPUS_ROUND changing inside the parent process.
func (s *HookSession) WriteRoundCursor(provider string, round int) error {
	if err := validateHookArtifactProvider(provider); err != nil {
		return err
	}
	if round <= 0 {
		return fmt.Errorf("hook round cursor must be positive: %d", round)
	}
	return s.writeJSONArtifact(hookRoundCursorName(provider), round)
}
