package regen

import (
	"errors"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// AC-QAMESH11-004: a synthesized maestro-scripted pack declaring AI authority is
// rejected by journey.Validate with code qa_journey_mobile_policy_invalid and is
// excluded from added/changed.
func TestEvaluatePack_MobileAIAuthorityRejectedByValidate(t *testing.T) {
	dir := t.TempDir()
	pack := mobileStarterPack()
	pack.PassFailAuthority = "ai"

	err := journey.Validate(pack, dir)
	var ve *journey.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *journey.ValidationError, got %v", err)
	}
	if ve.Code != "qa_journey_mobile_policy_invalid" {
		t.Fatalf("code = %q, want qa_journey_mobile_policy_invalid", ve.Code)
	}

	sp := EvaluatePack(dir, SurfaceMobile, pack)
	if !sp.Excluded {
		t.Fatal("mobile AI-authority pack must be excluded")
	}
	if sp.Reason != "qa_journey_mobile_policy_invalid" {
		t.Fatalf("reason = %q, want qa_journey_mobile_policy_invalid", sp.Reason)
	}

	diff := ComputeDiff([]SynthesizedPack{sp}, map[string]journey.Pack{})
	if diff.AddedCount != 0 || diff.ChangedCount != 0 {
		t.Fatalf("excluded pack leaked into diff: added=%d changed=%d", diff.AddedCount, diff.ChangedCount)
	}
}

// AC-QAMESH11-014: a web (gui-explore) pack and a desktop pack each declaring AI
// authority pass journey.Validate but are rejected by the AI-authority guard
// with qa_regen_ai_authority_forbidden, and appear in neither added nor changed.
func TestEvaluatePack_WebDesktopAIAuthorityRejectedByGuard(t *testing.T) {
	dir := t.TempDir()

	web := validWebPack("ai-web")
	web.PassFailAuthority = "ai"
	desktop := validDesktopPack("ai-desktop")
	desktop.PassFailAuthority = "ai"

	// journey.Validate does NOT reject web/desktop AI authority.
	if err := journey.Validate(web, dir); err != nil {
		t.Fatalf("web AI pack unexpectedly failed journey.Validate: %v", err)
	}
	if err := journey.Validate(desktop, dir); err != nil {
		t.Fatalf("desktop AI pack unexpectedly failed journey.Validate: %v", err)
	}

	for _, tc := range []struct {
		surface string
		pack    journey.Pack
	}{
		{SurfaceWeb, web},
		{SurfaceDesktop, desktop},
	} {
		code, allowed := AIAuthorityGuard(tc.pack)
		if allowed {
			t.Fatalf("%s AI pack must be rejected by guard", tc.surface)
		}
		if code != AIAuthorityForbiddenCode {
			t.Fatalf("guard code = %q, want %q", code, AIAuthorityForbiddenCode)
		}
		sp := EvaluatePack(dir, tc.surface, tc.pack)
		if !sp.Excluded || sp.Reason != AIAuthorityForbiddenCode {
			t.Fatalf("%s pack excluded=%v reason=%q", tc.surface, sp.Excluded, sp.Reason)
		}
	}

	diff := ComputeDiff(SynthesizeExtra(dir, SurfaceWeb, web, desktop), map[string]journey.Pack{})
	if diff.AddedCount != 0 || diff.ChangedCount != 0 {
		t.Fatalf("AI-authority packs leaked: added=%d changed=%d", diff.AddedCount, diff.ChangedCount)
	}
}

// Sanity: every synthesized starter pack passes journey.Validate so the diff is
// computed over real, valid packs.
func TestSynthesize_StarterPacksValidate(t *testing.T) {
	dir := t.TempDir()
	for _, surface := range []string{SurfaceWeb, SurfaceDesktop, SurfaceMobile} {
		pack, ok := synthesizeSurface(surface)
		if !ok {
			t.Fatalf("no starter pack for surface %q", surface)
		}
		mustValidate(t, pack, dir)
	}
}
