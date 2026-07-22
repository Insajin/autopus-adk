package orchestra

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

type hookCompletionProvenance string

const (
	hookCompletionUnknown          hookCompletionProvenance = "unknown"
	hookCompletionDone             hookCompletionProvenance = "done"
	hookCompletionNextRoundReady   hookCompletionProvenance = "next_round_ready"
	hookCompletionResponseFileOnly hookCompletionProvenance = "response_file_without_done"
)

type hookCompletionObservation struct {
	provenance hookCompletionProvenance
	round      int
}

// hookCompletionHealth tracks completion hooks that were configured but did
// not run in this orchestration. Its zero value is ready for concurrent use.
type hookCompletionHealth struct {
	mu       sync.RWMutex
	inactive map[string]hookCompletionObservation
}

// HasHook reports whether a configured completion hook is still active in this run.
func (s *HookSession) HasHook(provider string) bool {
	identity := providerArtifactIdentity(provider)
	s.hookHealth.mu.RLock()
	defer s.hookHealth.mu.RUnlock()
	_, inactive := s.hookHealth.inactive[identity]
	return s.hookProviders[identity] && !inactive
}

// SetHookProviders replaces completion hook configuration and resets run health.
func (s *HookSession) SetHookProviders(providers map[string]bool) {
	s.hookHealth.mu.Lock()
	defer s.hookHealth.mu.Unlock()
	s.hookProviders = make(map[string]bool, len(providers))
	for provider, enabled := range providers {
		s.hookProviders[providerArtifactIdentity(provider)] = enabled
	}
	s.hookHealth.inactive = nil
}

// completionArtifactProvenance checks evidence that the configured hook is
// active without mutating run health.
func (s *HookSession) completionArtifactProvenance(provider ProviderConfig, round int) (hookCompletionProvenance, error) {
	provenance, err := s.nextRoundReadyProvenance(provider, round)
	if err != nil || provenance == hookCompletionNextRoundReady {
		return provenance, err
	}
	doneName := RoundSignalName(provider.Name, round, "done")
	if _, err := s.statArtifact(doneName); err == nil {
		return hookCompletionDone, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return hookCompletionUnknown, fmt.Errorf("inspect completion artifact %s: %w", doneName, err)
	}
	return hookCompletionUnknown, nil
}

func (s *HookSession) nextRoundReadyProvenance(provider ProviderConfig, round int) (hookCompletionProvenance, error) {
	if err := validateHookArtifactProvider(provider.Name); err != nil {
		return hookCompletionUnknown, err
	}
	if !isCodexInteractiveProvider(provider) {
		return hookCompletionUnknown, nil
	}
	readyName := RoundSignalName(provider.Name, round+1, "ready")
	if _, err := s.statArtifact(readyName); err == nil {
		return hookCompletionNextRoundReady, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return hookCompletionUnknown, fmt.Errorf("inspect completion artifact %s: %w", readyName, err)
	}
	return hookCompletionUnknown, nil
}

// deactivateContinuationHook records stable idle evidence after a final ready
// check. A current done artifact proves execution, not a live next-round waiter.
func (s *HookSession) deactivateContinuationHook(provider ProviderConfig, round int) (hookCompletionProvenance, error) {
	provenance, err := s.nextRoundReadyProvenance(provider, round)
	if err != nil || provenance == hookCompletionNextRoundReady {
		return provenance, err
	}
	identity := providerArtifactIdentity(provider.Name)
	s.hookHealth.mu.Lock()
	if s.hookHealth.inactive == nil {
		s.hookHealth.inactive = make(map[string]hookCompletionObservation)
	}
	s.hookHealth.inactive[identity] = hookCompletionObservation{
		provenance: hookCompletionResponseFileOnly,
		round:      round,
	}
	s.hookHealth.mu.Unlock()
	return hookCompletionResponseFileOnly, nil
}
