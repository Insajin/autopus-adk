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
		if r, ok := respByName[name]; ok {
			out = append(out, classifyResponse(name, r))
			continue
		}
		if f, ok := failByName[name]; ok {
			note := f.FailureClass
			if note == "" {
				note = emptyNotePlaceholder
			}
			out = append(out, ProviderStatus{
				Provider: name,
				Status:   providerStatusError,
				Note:     note,
			})
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
	case r.ExitCode != 0 || r.Error != "":
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
func sanitizeNote(in string) string {
	if in == "" {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(in))
	for _, r := range in {
		// Replace any control char (newline, carriage return, tab, ASCII 0-31, DEL)
		// with a single space so the markdown table row stays one line.
		if r == '\n' || r == '\r' || r == '\t' || r < 0x20 || r == 0x7f {
			sb.WriteRune(' ')
			continue
		}
		// Pipe character would break the markdown table column boundary.
		if r == '|' {
			sb.WriteRune('/')
			continue
		}
		sb.WriteRune(r)
	}
	out := strings.TrimSpace(sb.String())
	if len(out) > noteMaxLen {
		out = out[:noteMaxLen] + "…"
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
