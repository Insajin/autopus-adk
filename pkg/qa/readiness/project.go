package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-007: Project is the redacted readiness projection authority for run, release, and evidence indexes.
// @AX:REASON: The CLI readiness command, backend validator, frontend CEO card, and desktop message card depend on this fail-closed JSON contract.
func Project(ctx context.Context, input Input) (Result, error) {
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	default:
	}
	normalizer := newWorkspacePathNormalizer(input.WorkspaceRoot)
	runDoc, err := readWorkspaceMap(input.RunIndexPath, normalizer)
	if err != nil {
		return Result{}, err
	}
	releaseDoc, err := readWorkspaceMap(input.ReleasePath, normalizer)
	if err != nil {
		return Result{}, err
	}
	if blocker := validateIndexes(input, runDoc, releaseDoc); blocker != "" {
		return failClosed(blocker)
	}
	for _, doc := range []map[string]any{runDoc, releaseDoc} {
		if class := unsafeClass(doc, ""); class != "" {
			return failClosed(class)
		}
	}
	runDocs, blocker := loadReleaseRunDocs(input, normalizer, runDoc, releaseDoc)
	if blocker != "" {
		return failClosed(blocker)
	}
	manifestPaths, blocker := safeManifestRefs(input.WorkspaceRoot, manifestRefsOf(runDocs))
	if blocker != "" {
		return failClosed(blocker)
	}
	manifestDocs, blocker, err := loadManifestDocs(input.WorkspaceRoot, normalizer, manifestPaths)
	if err != nil {
		return Result{}, err
	}
	if blocker != "" {
		return failClosed(blocker)
	}
	projection := buildProjection(runDocs, releaseDoc, manifestPaths, manifestDocs)
	return Result{
		Projection:           projection,
		Rendered:             renderFields(append([]map[string]any{runDoc, releaseDoc}, manifestDocs...)...),
		ProviderRepairPrompt: repairPrompt(runDoc, releaseDoc, firstManifest(manifestDocs)),
	}, nil
}

func validateIndexes(input Input, runDoc, releaseDoc map[string]any) string {
	if stringValue(runDoc["schema_version"]) != "qamesh.run_index.v1" {
		return "invalid_schema:run_index"
	}
	if stringValue(releaseDoc["schema_version"]) != "qamesh.release_index.v1" {
		return "invalid_schema:release_index"
	}
	if len(stringList(runDoc["source_refs"])) == 0 || len(stringList(releaseDoc["source_refs"])) == 0 {
		return "invalid_ref:missing_source_ref"
	}
	if !redactionPassed(runDoc["redaction_status"]) || !redactionPassed(releaseDoc["redaction_status"]) {
		return "unsafe_redaction:failed"
	}
	if !ownershipMatches(input, runDoc["workspace"]) || !ownershipMatches(input, releaseDoc["workspace"]) {
		return "invalid_owner:workspace_repo_mismatch"
	}
	return ""
}

func loadManifestDocs(root string, normalizer workspacePathNormalizer, paths []string) ([]map[string]any, string, error) {
	out := make([]map[string]any, 0, len(paths))
	for _, ref := range paths {
		doc, err := readWorkspaceMap(filepath.Join(root, ref), normalizer)
		if err != nil {
			return nil, "", err
		}
		if class := unsafeClass(doc, ""); class != "" {
			return nil, class, nil
		}
		schema := stringValue(doc["schema_version"])
		if schema != "qamesh.evidence.v1" && schema != "qamesh.evidence.v2" {
			return nil, "invalid_schema:evidence_manifest", nil
		}
		if !redactionPassed(doc["redaction_status"]) {
			return nil, "unsafe_redaction:manifest_failed", nil
		}
		out = append(out, doc)
	}
	return out, "", nil
}

// buildProjection describes the release, not just its newest run index.
// runDocs holds every run index the release references, primary first;
// manifestPaths is the ordered union of their manifest refs and pairs
// positionally with manifests.
func buildProjection(runDocs []map[string]any, releaseDoc map[string]any, manifestPaths []string, manifests []map[string]any) *Projection {
	primary := runDocs[0]
	lanes, laneStatuses := collectLanes(releaseDoc)
	feedbackRefs := collectFeedbackRefs(releaseDoc)
	evidenceRefs := collectEvidenceRefs(manifestPaths, manifests)
	counts := countChecks(runDocs)
	return &Projection{
		SchemaVersion:     ProjectionSchemaVersion,
		ContractOwner:     ContractOwner,
		ReadOnly:          true,
		ReleaseVerdict:    ReleaseVerdict(stringValue(releaseDoc["status"])),
		RawPayloadPresent: false,
		LaneStatuses:      laneStatuses,
		Lanes:             lanes,
		CheckCounts:       counts,
		SetupGaps:         collectSetupGaps(releaseDoc),
		DeferredLanes:     collectDeferredLanes(lanes),
		EvidenceRefs:      evidenceRefs,
		FeedbackRefs:      feedbackRefs,
		FeedbackActions:   collectFeedbackActions(primary, releaseDoc, manifests, evidenceRefs),
		SafeActions:       canonicalActions(),
		AuditRefs:         stringList(releaseDoc["audit_refs"]),
		SourceChains:      collectSourceChains(releaseDoc),
		LastRunTime:       stringValue(primary["ended_at"]),
		TrendSummary:      trendSummary(releaseDoc, lanes, counts),
	}
}

func collectLanes(releaseDoc map[string]any) ([]LaneStatus, map[string]Status) {
	rows := listOfMaps(releaseDoc["lane_rows"])
	lanes := make([]LaneStatus, 0, len(rows))
	statuses := map[string]Status{}
	for _, row := range rows {
		lane := stringValue(row["lane"])
		status := Status(stringValue(row["status"]))
		lanes = append(lanes, LaneStatus{Lane: lane, Status: status})
		statuses[lane] = status
	}
	return lanes, statuses
}

// countChecks totals the deterministic checks of every run index the release
// covers. Counting only the newest index reported one lane's checks as the
// whole release, directly contradicting the lane_statuses beside it.
func countChecks(runDocs []map[string]any) CheckCounts {
	counts := CheckCounts{}
	for _, runDoc := range runDocs {
		for _, check := range listOfMaps(runDoc["checks"]) {
			counts.Total++
			switch stringValue(check["status"]) {
			case "passed":
				counts.Passed++
			case "failed":
				counts.Failed++
			case "skipped":
				counts.Skipped++
			case "blocked":
				counts.Blocked++
			}
		}
	}
	return counts
}

func collectSetupGaps(releaseDoc map[string]any) []SetupGap {
	gaps := []SetupGap{}
	for _, row := range listOfMaps(releaseDoc["setup_gaps"]) {
		gaps = append(gaps, SetupGap{Lane: stringValue(row["lane"]), Class: stringValue(row["setup_gap_class"]), Reason: stringValue(row["reason"])})
	}
	return gaps
}

func collectDeferredLanes(lanes []LaneStatus) []string {
	out := []string{}
	for _, lane := range lanes {
		if lane.Status == StatusDeferred {
			out = append(out, lane.Lane)
		}
	}
	return out
}

// collectEvidenceRefs pairs each safe manifest ref with the manifest loaded
// from it. loadManifestDocs walks paths in order and fails closed on the first
// unreadable one, so index i always names manifests[i].
func collectEvidenceRefs(paths []string, manifests []map[string]any) []EvidenceRef {
	refs := make([]EvidenceRef, 0, len(paths))
	for i, path := range paths {
		qaID := ""
		if i < len(manifests) {
			qaID = stringValue(manifests[i]["qa_result_id"])
		}
		refs = append(refs, EvidenceRef{ManifestPath: path, QAResultID: qaID})
	}
	return refs
}

func collectFeedbackRefs(releaseDoc map[string]any) []FeedbackRef {
	refs := []FeedbackRef{}
	for _, path := range stringList(releaseDoc["feedback_refs"]) {
		refs = append(refs, FeedbackRef{BundlePath: path, Target: feedbackTarget(path)})
	}
	return refs
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: safe actions are read-only UI intents and must stay aligned with backend, frontend, and desktop validators.
func canonicalActions() []SafeAction {
	return []SafeAction{
		{Action: "open_evidence", ReadOnly: true},
		{Action: "open_feedback", ReadOnly: true},
		{Action: "rerun_lane", ReadOnly: true},
		{Action: "acknowledge_setup_gap", ReadOnly: true},
		{Action: "open_audit", ReadOnly: true},
	}
}

func trendSummary(releaseDoc map[string]any, lanes []LaneStatus, counts CheckCounts) string {
	if value := stringValue(releaseDoc["trend_summary"]); value != "" {
		return value
	}
	return fmt.Sprintf("%d failed deterministic check across %d lanes", counts.Failed, len(lanes))
}

// readWorkspaceMap decodes an evidence index and normalizes its in-workspace
// absolute paths before any caller inspects or classifies it, so a producer
// that ran with an absolute project dir does not poison the projection.
func readWorkspaceMap(path string, normalizer workspacePathNormalizer) (map[string]any, error) {
	doc, err := readMap(path)
	if err != nil {
		return nil, err
	}
	normalizer.normalizeDoc(doc)
	return doc, nil
}

// joinWorkspace resolves a workspace-relative ref against the workspace root.
func joinWorkspace(root, ref string) string {
	return filepath.Join(root, filepath.FromSlash(ref))
}

func failClosed(class string) (Result, error) {
	return Result{Blockers: []Blocker{{Class: class}}}, fmt.Errorf("readiness projection blocked: %s", class)
}

func readMap(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func firstManifest(manifests []map[string]any) map[string]any {
	if len(manifests) == 0 {
		return map[string]any{}
	}
	return manifests[0]
}

func feedbackTarget(path string) string {
	for target := range supportedTargets {
		if strings.Contains(path, "-"+target) || strings.Contains(path, "/"+target) {
			return target
		}
	}
	return ""
}
