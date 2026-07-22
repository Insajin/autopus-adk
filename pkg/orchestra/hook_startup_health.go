package orchestra

import "sync"

type hookStartupObservation struct {
	round int
}

// hookStartupHealth tracks startup hooks configured for this run that did not
// emit a ready artifact. It is independent from completion-hook health and its
// zero value is ready for concurrent use.
type hookStartupHealth struct {
	mu       sync.RWMutex
	inactive map[string]hookStartupObservation
}

// HasStartupHook reports whether startup-ready artifacts are still trusted for
// this run. Provider aliases share the generated artifact identity.
func (s *HookSession) HasStartupHook(provider string) bool {
	if s == nil {
		return false
	}
	identity := providerArtifactIdentity(provider)
	s.startupHealth.mu.RLock()
	defer s.startupHealth.mu.RUnlock()
	_, inactive := s.startupHealth.inactive[identity]
	return s.startupHookProviders[identity] && !inactive
}

// SetStartupHookProviders replaces startup-hook configuration and resets its
// run health without changing completion-hook configuration or health.
func (s *HookSession) SetStartupHookProviders(providers map[string]bool) {
	if s == nil {
		return
	}
	s.startupHealth.mu.Lock()
	defer s.startupHealth.mu.Unlock()
	s.startupHookProviders = make(map[string]bool, len(providers))
	for provider, enabled := range providers {
		s.startupHookProviders[providerArtifactIdentity(provider)] = enabled
	}
	s.startupHealth.inactive = nil
}

// deactivateCodexStartupHook records that this run cannot trust Codex's
// project-local SessionStart hook. Other providers remain fail-closed because
// their startup contracts do not have Codex's project-hook trust boundary.
func (s *HookSession) deactivateCodexStartupHook(provider string, round int) bool {
	if s == nil {
		return false
	}
	identity := providerArtifactIdentity(provider)
	if identity != "codex" {
		return false
	}
	s.startupHealth.mu.Lock()
	defer s.startupHealth.mu.Unlock()
	if !s.startupHookProviders[identity] {
		return false
	}
	if s.startupHealth.inactive == nil {
		s.startupHealth.inactive = make(map[string]hookStartupObservation)
	}
	s.startupHealth.inactive[identity] = hookStartupObservation{round: round}
	return true
}
