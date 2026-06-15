package regen

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// AIAuthorityForbiddenCode is the rejection reason code emitted when a pack
// declares AI pass/fail authority on any surface.
const AIAuthorityForbiddenCode = "qa_regen_ai_authority_forbidden"

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: security gate — journey.Validate only rejects AI authority for mobile adapters; this guard is the sole line of defense for web/desktop surfaces.
// @AX:REASON: weakening or removing this check silently permits AI-authority packs through the synthesis pipeline for web and desktop surfaces.
// AIAuthorityGuard rejects ANY pack that declares pass_fail_authority "ai",
// regardless of surface. journey.Validate only rejects AI authority for the
// two mobile adapters (maestro-scripted, appium-mobile-explore); web and
// desktop packs pass journey.Validate clean, so this guard is the only line of
// defense for those surfaces. Returns ("", true) when the pack is allowed and
// (AIAuthorityForbiddenCode, false) when it must be excluded.
func AIAuthorityGuard(pack journey.Pack) (reasonCode string, allowed bool) {
	if strings.EqualFold(strings.TrimSpace(pack.PassFailAuthority), "ai") {
		return AIAuthorityForbiddenCode, false
	}
	return "", true
}
