package regen

import (
	"errors"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// Synthesize builds one journey.Pack per present surface (structural mirror of
// the scaffold starter templates, NOT behavioral extraction), then runs each
// pack through journey.Validate and the surface-agnostic AI-authority guard.
//
// Validation order per pack:
//  1. journey.Validate — catches mobile AI authority (qa_journey_mobile_policy_invalid)
//     and any other structural/policy violation.
//  2. AIAuthorityGuard — catches AI authority on web/desktop, which journey.Validate
//     permits (qa_regen_ai_authority_forbidden).
//
// A pack failing either gate is marked Excluded with the reason code; excluded
// packs never enter the added/changed sets in the diff.
func Synthesize(projectDir string, surfaces []string) []SynthesizedPack {
	result := make([]SynthesizedPack, 0, len(surfaces))
	for _, surface := range surfaces {
		pack, ok := synthesizeSurface(surface)
		if !ok {
			continue
		}
		result = append(result, evaluatePack(projectDir, surface, pack))
	}
	return result
}

// SynthesizeExtra evaluates explicitly-provided packs (e.g. test fixtures or
// flows discovered outside the fixed surface templates) through the same two
// gates and tags them with the given surface.
func SynthesizeExtra(projectDir, surface string, packs ...journey.Pack) []SynthesizedPack {
	result := make([]SynthesizedPack, 0, len(packs))
	for _, pack := range packs {
		result = append(result, evaluatePack(projectDir, surface, pack))
	}
	return result
}

// EvaluatePack runs a single pack through journey.Validate and the AI-authority
// guard, returning the SynthesizedPack with Excluded/Reason populated. Exported
// so Unit 2 can re-evaluate packs it sources independently.
func EvaluatePack(projectDir, surface string, pack journey.Pack) SynthesizedPack {
	return evaluatePack(projectDir, surface, pack)
}

func evaluatePack(projectDir, surface string, pack journey.Pack) SynthesizedPack {
	sp := SynthesizedPack{Pack: pack, Surface: surface}
	if err := journey.Validate(pack, projectDir); err != nil {
		sp.Excluded = true
		sp.Reason = validationReason(err)
		return sp
	}
	if code, allowed := AIAuthorityGuard(pack); !allowed {
		sp.Excluded = true
		sp.Reason = code
		return sp
	}
	return sp
}

func synthesizeSurface(surface string) (journey.Pack, bool) {
	switch surface {
	case SurfaceWeb:
		return webStarterPack(), true
	case SurfaceDesktop:
		return desktopStarterPack(), true
	case SurfaceMobile:
		return mobileStarterPack(), true
	default:
		return journey.Pack{}, false
	}
}

// validationReason extracts the *journey.ValidationError code when present,
// falling back to the error string for non-validation errors.
func validationReason(err error) string {
	var ve *journey.ValidationError
	if errors.As(err, &ve) {
		return ve.Code
	}
	return err.Error()
}
