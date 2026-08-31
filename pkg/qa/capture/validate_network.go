package capture

import (
	"fmt"
	"strings"
)

func validateNetwork(position int, network *NetworkSummary) error {
	if network == nil {
		return nil
	}
	if len(network.Entries) > MaxNetworkPerStep {
		return fmt.Errorf("steps[%d].network.entries exceed %d entries", position, MaxNetworkPerStep)
	}
	for index, entry := range network.Entries {
		if !httpMethodRe.MatchString(entry.Method) {
			return fmt.Errorf("steps[%d].network.entries[%d].method %q must be an uppercase HTTP method", position, index, entry.Method)
		}
		if err := validateURLRef(entry.URLRef); err != nil {
			return fmt.Errorf("steps[%d].network.entries[%d]: %w", position, index, err)
		}
		if entry.Status != 0 && (entry.Status < minRecognizedHTTPStatus || entry.Status > maxHTTPStatus) {
			return fmt.Errorf("steps[%d].network.entries[%d].status %d is out of range", position, index, entry.Status)
		}
	}
	return nil
}

// validateURLRef enforces that network evidence carries a reference, not a URL.
// An absolute URL can smuggle credentials, tokens, and query secrets past the
// redaction scan, so the shape itself is rejected.
func validateURLRef(value string) error {
	ref := strings.TrimSpace(value)
	if ref == "" {
		return fmt.Errorf("url_ref is required")
	}
	if len(ref) > MaxRefLen {
		return fmt.Errorf("url_ref exceeds %d characters", MaxRefLen)
	}
	if strings.Contains(ref, "://") || strings.Contains(ref, "@") {
		return fmt.Errorf("url_ref must not contain an absolute URL")
	}
	// A protocol-relative ref resolves to a different host in a browser, so it is
	// an absolute URL wearing a relative-looking prefix.
	if strings.HasPrefix(ref, "//") {
		return fmt.Errorf("url_ref must not be protocol-relative")
	}
	if strings.ContainsAny(ref, "?#") {
		return fmt.Errorf("url_ref must not contain a query or fragment")
	}
	if strings.HasPrefix(ref, "origin:") {
		if !originRefRe.MatchString(ref) {
			return fmt.Errorf("url_ref must match origin:<index>[/path]")
		}
		return nil
	}
	if !strings.HasPrefix(ref, "/") {
		return fmt.Errorf("url_ref must be origin-relative or origin:<index>[/path]")
	}
	return nil
}
