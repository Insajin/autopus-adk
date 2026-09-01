package readiness

// loadReleaseRunDocs returns every run index the release describes, primary
// first, de-duplicated by run id.
//
// auto qa release writes one run index per lane, so a projection built from the
// single newest index under-reports every multi-lane release and contradicts
// the lane_statuses carried in the same payload. Each sibling index is
// validated and scanned exactly like the primary: a release that references
// evidence this package would refuse to project on its own must not become
// projectable by aggregation.
func loadReleaseRunDocs(input Input, normalizer workspacePathNormalizer, runDoc, releaseDoc map[string]any) ([]map[string]any, string) {
	rawRefs, blocker := releaseRunIndexRefs(releaseDoc)
	if blocker != "" {
		return nil, blocker
	}
	refs, blocker := safeWorkspaceRefs(input.WorkspaceRoot, refKindRunIndex, rawRefs)
	if blocker != "" {
		return nil, blocker
	}
	docs := []map[string]any{runDoc}
	seen := map[string]bool{stringValue(runDoc["run_id"]): true}
	for _, ref := range refs {
		doc, err := readWorkspaceMap(joinWorkspace(input.WorkspaceRoot, ref), normalizer)
		if err != nil {
			// The release names this index as its own evidence, so an
			// unreadable one is a broken release, not a missing optional file.
			return nil, "invalid_ref:run_index_unreadable"
		}
		if id := stringValue(doc["run_id"]); id == "" || seen[id] {
			continue
		}
		if class := validateRunIndexDoc(input, doc); class != "" {
			return nil, class
		}
		if class := unsafeClass(doc, ""); class != "" {
			return nil, class
		}
		seen[stringValue(doc["run_id"])] = true
		docs = append(docs, doc)
	}
	return docs, ""
}

// releaseRunIndexRefs collects the run-index refs named by the release lane
// rows in lane order.
//
// A passed, failed, or warn lane verdict can only come from a lane that ran, so
// a row carrying one without naming its run index describes evidence the
// projection cannot reach. Skipping those rows is what let check_counts report
// a single lane's checks as the whole release, so an incomplete release index
// fails closed instead. Lanes that never ran (deferred, skipped, blocked,
// setup_gap) legitimately carry no ref.
func releaseRunIndexRefs(releaseDoc map[string]any) ([]string, string) {
	rows := listOfMaps(releaseDoc["lane_rows"])
	refs := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		ref := stringValue(row["run_index_path"])
		if ref == "" {
			if executedLaneStatuses[Status(stringValue(row["status"]))] {
				return nil, "invalid_ref:lane_run_index_missing"
			}
			continue
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs, ""
}

// executedLaneStatuses are the lane verdicts only an executed lane can produce.
var executedLaneStatuses = map[Status]bool{
	StatusPassed: true,
	StatusFailed: true,
	StatusWarn:   true,
}

// validateRunIndexDoc applies the primary run index's schema, provenance,
// redaction, and ownership gates to a sibling index.
func validateRunIndexDoc(input Input, doc map[string]any) string {
	if stringValue(doc["schema_version"]) != "qamesh.run_index.v1" {
		return "invalid_schema:run_index"
	}
	if len(stringList(doc["source_refs"])) == 0 {
		return "invalid_ref:missing_source_ref"
	}
	if !redactionPassed(doc["redaction_status"]) {
		return "unsafe_redaction:failed"
	}
	if !ownershipMatches(input, doc["workspace"]) {
		return "invalid_owner:workspace_repo_mismatch"
	}
	return ""
}

// manifestRefsOf returns the ordered union of manifest refs across every run
// index of the release, so evidence_refs and feedback actions describe the same
// release the lane statuses do.
func manifestRefsOf(runDocs []map[string]any) []string {
	refs := []string{}
	seen := map[string]bool{}
	for _, doc := range runDocs {
		for _, ref := range stringList(doc["manifest_paths"]) {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	return refs
}
