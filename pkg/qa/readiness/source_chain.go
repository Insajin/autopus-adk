package readiness

func collectSourceChains(releaseDoc map[string]any) []SourceChain {
	releaseIndexPath := releaseIndexPath(releaseDoc)
	auditRef := firstString(stringList(releaseDoc["audit_refs"]))
	sourceRefs := stringList(releaseDoc["source_refs"])
	rows := listOfMaps(releaseDoc["lane_rows"])
	if len(rows) == 0 {
		return []SourceChain{{
			ReleaseIndexPath: releaseIndexPath,
			AuditRef:         auditRef,
			SourceRefs:       sourceRefs,
		}}
	}
	out := make([]SourceChain, 0, len(rows))
	for _, row := range rows {
		manifestPaths := stringList(row["manifest_paths"])
		feedbackRefs := stringList(row["feedback_refs"])
		maxLen := maxInt(1, len(manifestPaths), len(feedbackRefs))
		for i := 0; i < maxLen; i++ {
			out = append(out, SourceChain{
				Lane:             stringValue(row["lane"]),
				ReleaseIndexPath: releaseIndexPath,
				RunIndexPath:     stringValue(row["run_index_path"]),
				ManifestPath:     indexedString(manifestPaths, i),
				FeedbackBundle:   indexedString(feedbackRefs, i),
				AuditRef:         auditRef,
				SourceRefs:       sourceRefs,
			})
		}
	}
	return out
}

func releaseIndexPath(releaseDoc map[string]any) string {
	outputs, ok := releaseDoc["output_paths"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(outputs["release_index_path"])
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func indexedString(values []string, index int) string {
	if index < 0 || index >= len(values) {
		return ""
	}
	return values[index]
}

func maxInt(values ...int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
