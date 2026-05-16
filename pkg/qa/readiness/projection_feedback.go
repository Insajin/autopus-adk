package readiness

func collectFeedbackActions(runDoc, releaseDoc map[string]any, manifests []map[string]any, refs []EvidenceRef) []FeedbackAction {
	if len(refs) == 0 {
		return []FeedbackAction{{Target: "codex", Enabled: false, DisabledReason: "missing_evidence"}}
	}
	actions := make([]FeedbackAction, 0, len(refs))
	for i, ref := range refs {
		var manifest map[string]any
		if i < len(manifests) {
			manifest = manifests[i]
		}
		actions = append(actions, DeriveFeedbackAction(EvidenceForFeedback{
			Status:                 evidenceStatus(runDoc, manifest),
			DeterministicAuthority: deterministicAuthority(releaseDoc, manifest, ref.ManifestPath),
			RedactionStatus:        combinedRedactionStatus(runDoc, releaseDoc, manifest),
			ManifestPath:           ref.ManifestPath,
		}, feedbackActionTarget(runDoc, releaseDoc)))
	}
	return actions
}

func evidenceStatus(runDoc, manifest map[string]any) Status {
	if manifest != nil {
		if status := Status(stringValue(manifest["status"])); status != "" {
			return status
		}
	}
	return Status(stringValue(runDoc["status"]))
}

func deterministicAuthority(releaseDoc, manifest map[string]any, manifestPath string) bool {
	lane := stringValue(manifest["lane"])
	for _, row := range listOfMaps(releaseDoc["lane_rows"]) {
		if lane != "" && stringValue(row["lane"]) == lane {
			return boolValue(row["deterministic_authority"])
		}
	}
	for _, row := range listOfMaps(releaseDoc["lane_rows"]) {
		if stringListContains(stringList(row["manifest_paths"]), manifestPath) {
			return boolValue(row["deterministic_authority"])
		}
	}
	return boolValue(releaseDoc["deterministic_authority"])
}

func combinedRedactionStatus(docs ...map[string]any) RedactionStatus {
	for _, doc := range docs {
		if doc != nil && !redactionPassed(doc["redaction_status"]) {
			return RedactionFailed
		}
	}
	return RedactionPassed
}

func feedbackActionTarget(runDoc, releaseDoc map[string]any) string {
	for _, ref := range append(stringList(releaseDoc["feedback_refs"]), stringList(runDoc["feedback_bundle_paths"])...) {
		if target := feedbackTarget(ref); target != "" {
			return target
		}
	}
	return "codex"
}

func stringListContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
