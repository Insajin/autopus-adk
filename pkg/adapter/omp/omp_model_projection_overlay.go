package omp

import "fmt"

// OMPModelOverlayFromProjection is the explicit bridge into config activation.
// It returns detached maps so activation cannot mutate the compiled projection.
func OMPModelOverlayFromProjection(
	projection OMPModelProjection,
) (OMPModelOverlayProjection, error) {
	roles, err := indexOMPModelRoleProjection(projection.ModelRoles)
	if err != nil {
		return OMPModelOverlayProjection{}, err
	}
	overlay := OMPModelOverlayProjection{
		ModelRoles:     roles,
		FallbackChains: make(map[string][]string, len(projection.FallbackChains)),
	}
	for _, chain := range projection.FallbackChains {
		selector, thinking, splitErr := splitOMPProjectedSelector(chain.Selector)
		if splitErr != nil {
			return OMPModelOverlayProjection{}, splitErr
		}
		if validateErr := validateOMPProjectedSelector(selector, thinking); validateErr != nil {
			return OMPModelOverlayProjection{}, validateErr
		}
		if _, duplicate := overlay.FallbackChains[chain.Selector]; duplicate {
			return OMPModelOverlayProjection{}, fmt.Errorf("fallback_chain_duplicate: %s", chain.Selector)
		}
		for _, candidate := range chain.Candidates {
			candidateSelector, candidateThinking, candidateErr := splitOMPProjectedSelector(candidate)
			if candidateErr != nil {
				return OMPModelOverlayProjection{}, candidateErr
			}
			if validateErr := validateOMPProjectedSelector(candidateSelector, candidateThinking); validateErr != nil {
				return OMPModelOverlayProjection{}, validateErr
			}
		}
		overlay.FallbackChains[chain.Selector] = append([]string(nil), chain.Candidates...)
	}
	return overlay, nil
}
