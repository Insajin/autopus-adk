package run

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// evaluateGUINetworkEvidence resolves the allowed-origin evidence the runtime
// policy oracle needs.
//
// When a pack declares the typed capture network stream, the capture index is
// authoritative: its `url_ref` values are already validated to be
// origin-relative or `origin:<index>`, which is a stronger guarantee than
// scanning free-form URLs out of a producer-authored network_summary. Packs
// without capture keep the declared-artifact path unchanged.
func evaluateGUINetworkEvidence(projectDir string, pack journey.Pack, result *commandResult) (outside, missing []string) {
	if pack.GUI.Capture.HasStream(capture.StreamNetwork) {
		return captureNetworkOriginFindings(pack, result)
	}
	network, err := readDeclaredJSON(projectDir, pack, "network_summary")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return outsideAllowedNetworkRequests(network, pack.GUI.AllowedOrigins), nil
}

// captureNetworkOriginFindings verifies every captured request reference resolves
// to a declared origin. An `origin:<index>` pointing past the allowlist means the
// producer captured traffic the pack never authorized.
func captureNetworkOriginFindings(pack journey.Pack, result *commandResult) (outside, missing []string) {
	index, err := loadCaptureIndexOnce(result)
	if err != nil {
		return nil, []string{"capture_index.network: " + err.Error()}
	}
	origins := cleanedList(pack.GUI.AllowedOrigins)
	seen := map[string]bool{}
	for _, step := range index.Steps {
		if step.Network == nil {
			continue
		}
		for _, entry := range step.Network.Entries {
			finding := originFindingFor(entry.URLRef, len(origins))
			if finding == "" || seen[finding] {
				continue
			}
			seen[finding] = true
			outside = append(outside, finding)
		}
	}
	sort.Strings(outside)
	return outside, nil
}

// originFindingFor returns a finding string when a reference does not resolve to
// a declared origin, or "" when it is in policy.
func originFindingFor(ref string, originCount int) string {
	value := strings.TrimSpace(ref)
	if strings.HasPrefix(value, "/") {
		// Same-origin relative reference: in policy by construction.
		return ""
	}
	if !strings.HasPrefix(value, "origin:") {
		return "unresolvable_url_ref:" + value
	}
	digits := strings.TrimPrefix(value, "origin:")
	if slash := strings.Index(digits, "/"); slash >= 0 {
		digits = digits[:slash]
	}
	position, err := strconv.Atoi(digits)
	if err != nil || position < 0 || position >= originCount {
		return fmt.Sprintf("origin_out_of_policy:%s", value)
	}
	return ""
}

// loadCaptureIndexOnce memoizes the parsed index so the policy oracle and the
// capture oracle do not each re-read and re-validate the same file.
func loadCaptureIndexOnce(result *commandResult) (capture.Index, error) {
	if result.captureIndex != nil {
		return *result.captureIndex, nil
	}
	if strings.TrimSpace(result.CaptureIndexPath) == "" {
		return capture.Index{}, fmt.Errorf("capture runtime was not prepared")
	}
	index, err := capture.LoadIndex(result.CaptureIndexPath)
	if err != nil {
		return capture.Index{}, err
	}
	if err := capture.Validate(index); err != nil {
		return capture.Index{}, err
	}
	result.captureIndex = &index
	return index, nil
}
