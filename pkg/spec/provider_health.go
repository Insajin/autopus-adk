package spec

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// SPEC-SPECREV-001 REQ-VERD-1 / REQ-VERD-2 / REQ-VERD-4 helpers.
// Provider Health classification, degradation labeling, and table rendering.

const (
	// providerStatusSuccess marks a provider that returned a usable response.
	providerStatusSuccess = "success"
	// providerStatusTimeout marks a provider that timed out before responding.
	providerStatusTimeout = "timeout"
	// providerStatusError marks a provider that exited non-zero or failed preflight.
	providerStatusError = "error"

	// degradedThreshold is the inclusive failure ratio that triggers the
	// "(degraded — N/M providers responded)" label (REQ-VERD-2).
	// @AX:NOTE: [AUTO] magic constant — 0.5 is the inclusive REQ-VERD-2 threshold; boundary is >= not >
	degradedThreshold = 0.5

	// emptyNotePlaceholder renders in the Note column when the upstream value
	// is empty so the markdown table stays well-formed.
	emptyNotePlaceholder = "-"
)

// ClassifyProviderStatuses is a deterministic pass-through that preserves
// caller-supplied ordering and per-row Status values. It exists so that the
// review pipeline has a single seam through which provider statuses flow into
// formatReviewMd, even when callers pre-compute the slice.
func ClassifyProviderStatuses(statuses []ProviderStatus) []ProviderStatus {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]ProviderStatus, len(statuses))
	for i, s := range statuses {
		if s.Note == "" {
			s.Note = emptyNotePlaceholder
		}
		out[i] = s
	}
	return out
}

// BuildProviderStatuses converts orchestra responses + failed providers into a
// deterministic ProviderStatus slice ordered by `configured`. Providers absent
// from both inputs are reported as errors with a "no response" note so the
// review.md table never silently drops a configured provider.
func BuildProviderStatuses(
	responses []orchestra.ProviderResponse,
	failed []orchestra.FailedProvider,
	configured []string,
) []ProviderStatus {
	respByName := make(map[string]orchestra.ProviderResponse, len(responses))
	for _, r := range responses {
		respByName[r.Provider] = r
	}
	failByName := make(map[string]orchestra.FailedProvider, len(failed))
	for _, f := range failed {
		failByName[f.Name] = f
	}

	out := make([]ProviderStatus, 0, len(configured))
	for _, name := range configured {
		if f, ok := failByName[name]; ok {
			out = append(out, classifyFailedProvider(name, f))
			continue
		}
		if r, ok := respByName[name]; ok {
			out = append(out, classifyResponse(name, r))
			continue
		}
		out = append(out, ProviderStatus{
			Provider: name,
			Status:   providerStatusError,
			Note:     "no response",
		})
	}
	return out
}

func classifyFailedProvider(name string, f orchestra.FailedProvider) ProviderStatus {
	status := providerStatusError
	if f.FailureClass == providerStatusTimeout {
		status = providerStatusTimeout
	}
	note := f.FailureClass
	if note == "" {
		note = emptyNotePlaceholder
	}
	return ProviderStatus{
		Provider: name,
		Status:   status,
		Note:     note,
	}
}

// noteMaxLen bounds Note length so provider stderr (which can carry stack
// traces or absolute filesystem paths) does not become a permanent artifact in
// committed review.md files.
// @AX:NOTE: [AUTO] hardening boundary — provider Error strings are sanitized before reaching review.md to prevent leakage of paths/credentials into git history (S-001).
const noteMaxLen = 200

// classifyResponse maps an orchestra ProviderResponse into a ProviderStatus.
// Error notes are sanitized: control characters stripped, length capped at
// noteMaxLen so committed review.md never embeds raw provider stderr.
func classifyResponse(name string, r orchestra.ProviderResponse) ProviderStatus {
	switch {
	case r.TimedOut:
		return ProviderStatus{Provider: name, Status: providerStatusTimeout, Note: emptyNotePlaceholder}
	case r.ExitCode != 0:
		note := sanitizeNote(r.Error)
		if note == "" {
			note = fmt.Sprintf("exit=%d", r.ExitCode)
		}
		return ProviderStatus{Provider: name, Status: providerStatusError, Note: note}
	default:
		return ProviderStatus{Provider: name, Status: providerStatusSuccess, Note: emptyNotePlaceholder}
	}
}

// sanitizeNote strips control characters and truncates the input so untrusted
// provider stderr cannot inject markdown-breaking characters or leak long
// paths/credentials into committed review.md.
//
// Truncation is rune-aware so multi-byte UTF-8 sequences at the cap boundary
// are never split, preventing malformed runes in committed review.md.
func sanitizeNote(in string) string {
	if in == "" {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(in))
	count := 0
	truncated := false
	for _, r := range in {
		// Replace any control char (newline, carriage return, tab, ASCII 0-31, DEL)
		// with a single space so the markdown table row stays one line.
		if r == '\n' || r == '\r' || r == '\t' || r < 0x20 || r == 0x7f {
			r = ' '
		} else if r == '|' {
			// Pipe character would break the markdown table column boundary.
			r = '/'
		}
		if count >= noteMaxLen {
			truncated = true
			break
		}
		sb.WriteRune(r)
		count++
	}
	out := strings.TrimSpace(sb.String())
	if truncated {
		out += "…"
	}
	return out
}

// ShouldLabelDegraded reports whether the (failed / totalConfigured) ratio
// reaches the inclusive 50% threshold mandated by REQ-VERD-2. Failure is any
// status other than "success".
// @AX:NOTE: [AUTO] subtle invariant — providers absent from statuses slice (missing < 0) count as failures; totalConfigured is the source of truth for denominator
func ShouldLabelDegraded(statuses []ProviderStatus, totalConfigured int) bool {
	if totalConfigured <= 0 {
		return false
	}
	failed := 0
	for _, s := range statuses {
		if s.Status != providerStatusSuccess {
			failed++
		}
	}
	// Account for configured providers missing from the slice entirely (they
	// could not respond and are therefore failures by definition).
	if missing := totalConfigured - len(statuses); missing > 0 {
		failed += missing
	}
	ratio := float64(failed) / float64(totalConfigured)
	return ratio >= degradedThreshold
}

// CountProviderStatus returns the number of statuses matching `target`. It is
// used both for the degraded label numerator and for tests / loop callers.
func CountProviderStatus(statuses []ProviderStatus, target string) int {
	n := 0
	for _, s := range statuses {
		if s.Status == target {
			n++
		}
	}
	return n
}

// DegradedLabel renders the "(degraded — N/M providers responded)" suffix
// where N is the success count and M is the configured provider count. It
// returns the empty string when the suffix is not warranted.
func DegradedLabel(statuses []ProviderStatus, totalConfigured int) string {
	if totalConfigured <= 0 || !ShouldLabelDegraded(statuses, totalConfigured) {
		return ""
	}
	n := CountProviderStatus(statuses, providerStatusSuccess)
	return fmt.Sprintf(" (degraded — %d/%d providers responded)", n, totalConfigured)
}

// SPEC-ADK-REVIEW-INTEGRITY-001 REQ-RINT-QUORUM-05 quorum policy.
// How many providers must return a usable review before the review gate may
// auto-promote a SPEC to "approved". These reuse the existing ProviderStatus /
// CountProviderStatus machinery instead of duplicating provider bookkeeping.

// DegradedReasonProviderQuorum is the machine-readable degraded reason recorded
// on the verdict when the usable-provider count falls below the quorum. The
// promotion gate reads this code to block auto-promotion (REQ-RINT-QUORUM-05).
const DegradedReasonProviderQuorum = "provider_quorum"

// DefaultMinProviders returns the majority quorum for a configured provider
// count: configured/2 + 1 (integer division). This keeps single-provider local
// reviews passing (n=1 -> 1) while blocking a 1-of-3 promotion (n=3 -> 2). A
// non-positive count is treated as requiring one usable review (fail-closed).
// @AX:NOTE: [AUTO] magic constant — configured/2 + 1 encodes "majority"; dropping the +1 would let n=2 pass at quorum=1 and silently weaken REQ-RINT-QUORUM-05
func DefaultMinProviders(configured int) int {
	if configured <= 0 {
		return 1
	}
	return configured/2 + 1
}

// EffectiveMinProviders resolves the configured minimum against the provider
// count. A configured value <= 0 means "unset" and derives the majority default
// via DefaultMinProviders; a positive value is used verbatim as an operator
// override.
func EffectiveMinProviders(configured, configuredMin int) int {
	if configuredMin > 0 {
		return configuredMin
	}
	return DefaultMinProviders(configured)
}

// MeetsProviderQuorum reports whether the number of providers that returned a
// usable ("success") review meets the effective minimum quorum. When the quorum
// is not met it returns a human-readable detail string ("providers N/M < quorum
// Q") suitable for surfacing alongside DegradedReasonProviderQuorum; when met
// the detail string is empty.
func MeetsProviderQuorum(statuses []ProviderStatus, configured, minProviders int) (bool, string) {
	quorum := EffectiveMinProviders(configured, minProviders)
	responded := CountProviderStatus(statuses, providerStatusSuccess)
	if responded >= quorum {
		return true, ""
	}
	return false, fmt.Sprintf("providers %d/%d < quorum %d", responded, configured, quorum)
}

// RenderProviderHealthSection renders the markdown section that documents
// per-provider Status and Note. The exact heading and column order are pinned
// by acceptance.md AC-VERD-1.
func RenderProviderHealthSection(statuses []ProviderStatus, totalConfigured int) string {
	if len(statuses) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Provider Health\n\n")
	sb.WriteString("| Provider | Status | Note |\n")
	sb.WriteString("| --- | --- | --- |\n")
	for _, s := range statuses {
		note := s.Note
		if note == "" {
			note = emptyNotePlaceholder
		}
		fmt.Fprintf(&sb, "| %s | %s | %s |\n", s.Provider, s.Status, note)
	}
	sb.WriteString("\n")
	return sb.String()
}
