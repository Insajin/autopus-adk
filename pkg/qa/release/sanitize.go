package release

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: release index sanitization is the boundary before JSON persistence and CLI payloads.
// @AX:REASON: downgraded from ANCHOR during sync; grep fan-in is below threshold, but output paths, blockers, feedback refs, and AI refs still share this redaction boundary.
func sanitizeIndex(index Index) (Index, RedactionState) {
	changed := false
	redact := func(value string) string {
		next, didChange := redactReleaseString(value)
		if didChange {
			changed = true
		}
		return next
	}
	index.ReleaseID = redact(index.ReleaseID)
	index.Profile = redact(index.Profile)
	index.StartedAt = redact(index.StartedAt)
	index.EndedAt = redact(index.EndedAt)
	index.OutputPaths = sanitizeOutputPaths(index.OutputPaths, redact)
	index.SourceRefs = sanitizeStrings(index.SourceRefs, redact)
	for i := range index.SetupGaps {
		index.SetupGaps[i].Lane = redact(index.SetupGaps[i].Lane)
		index.SetupGaps[i].Reason = redact(index.SetupGaps[i].Reason)
		index.SetupGaps[i].OwnerSpec = redact(index.SetupGaps[i].OwnerSpec)
		index.SetupGaps[i].OwnerRepo = redact(index.SetupGaps[i].OwnerRepo)
	}
	for i := range index.SiblingSpecs {
		index.SiblingSpecs[i].SpecID = redact(index.SiblingSpecs[i].SpecID)
		index.SiblingSpecs[i].OwnerRepo = redact(index.SiblingSpecs[i].OwnerRepo)
		index.SiblingSpecs[i].Lanes = sanitizeStrings(index.SiblingSpecs[i].Lanes, redact)
		index.SiblingSpecs[i].Status = redact(index.SiblingSpecs[i].Status)
		index.SiblingSpecs[i].Relationship = redact(index.SiblingSpecs[i].Relationship)
	}
	for i := range index.LaneRows {
		index.LaneRows[i] = sanitizeLaneRow(index.LaneRows[i], redact)
	}
	for i := range index.Blockers {
		index.Blockers[i].Lane = redact(index.Blockers[i].Lane)
		index.Blockers[i].Reason = redact(index.Blockers[i].Reason)
	}
	index.SelectedLanes = sanitizeStrings(index.SelectedLanes, redact)
	index.FeedbackRefs = sanitizeStrings(index.FeedbackRefs, redact)
	for i := range index.AIAnalysisRefs {
		index.AIAnalysisRefs[i].Ref = redact(index.AIAnalysisRefs[i].Ref)
	}
	if changed {
		index.RedactionStatus = mergeRedaction(index.RedactionStatus, RedactionRedacted)
		return index, RedactionRedacted
	}
	return index, RedactionClean
}

func sanitizeLaneRow(row LaneRow, redact func(string) string) LaneRow {
	row.Lane = redact(row.Lane)
	row.OwnerSpec = redact(row.OwnerSpec)
	row.OwnerRepo = redact(row.OwnerRepo)
	row.RunIndexPath = redact(row.RunIndexPath)
	row.ManifestPaths = sanitizeStrings(row.ManifestPaths, redact)
	row.FeedbackRefs = sanitizeStrings(row.FeedbackRefs, redact)
	row.FailedJourneyID = redact(row.FailedJourneyID)
	row.FailureSummary = redact(row.FailureSummary)
	row.SkippedReason = redact(row.SkippedReason)
	for i := range row.Blockers {
		row.Blockers[i].Lane = redact(row.Blockers[i].Lane)
		row.Blockers[i].Reason = redact(row.Blockers[i].Reason)
	}
	return row
}

func sanitizeOutputPaths(paths OutputPaths, redact func(string) string) OutputPaths {
	paths.ReleaseIndexPreviewPath = redact(paths.ReleaseIndexPreviewPath)
	paths.ReleaseIndexPath = redact(paths.ReleaseIndexPath)
	paths.RunIndexRoot = redact(paths.RunIndexRoot)
	paths.EvidenceRoot = redact(paths.EvidenceRoot)
	paths.FeedbackRoot = redact(paths.FeedbackRoot)
	return paths
}

func sanitizeStrings(values []string, redact func(string) string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = redact(value)
	}
	return out
}
