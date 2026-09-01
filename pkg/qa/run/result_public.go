package run

import "time"

// publishResult writes the run index to its real on-disk location and returns
// the public form of the result.
//
// The published refs are project-root-relative. That is the fix for the whole
// defect class: an absolute --project-dir used to produce absolute refs that
// downstream private-path redaction then had to remove, which erased the
// evidence chain of lanes that actually ran. With relative refs there is no
// private path left to redact, and the output is identical no matter how
// --project-dir was spelled.
func publishResult(result Result, opts Options, started, ended time.Time) (Result, error) {
	// The write target must stay the real path; only what we publish is
	// rewritten.
	destination := result.RunIndexPath
	public := publicResult(result, opts.ProjectDir)
	if err := writeIndex(public, destination, opts, started, ended); err != nil {
		return public, err
	}
	return public, nil
}

func publicResult(result Result, projectDir string) Result {
	result.OutputRoot = PublicProjectPath(projectDir, result.OutputRoot)
	result.RunIndexPath = PublicProjectPath(projectDir, result.RunIndexPath)
	result.RunIndexPreviewPath = PublicProjectPath(projectDir, result.RunIndexPreviewPath)
	result.ManifestPaths = publicProjectPaths(projectDir, result.ManifestPaths)
	result.ManifestPreviews = publicProjectPaths(projectDir, result.ManifestPreviews)
	result.FeedbackBundlePaths = publicProjectPaths(projectDir, result.FeedbackBundlePaths)
	for i := range result.AdapterResults {
		result.AdapterResults[i].QAMESHManifestPath = PublicProjectPath(
			projectDir, result.AdapterResults[i].QAMESHManifestPath,
		)
	}
	for i := range result.ArtifactPreviews {
		result.ArtifactPreviews[i].Path = publicPreviewPath(projectDir, result.ArtifactPreviews[i].Path)
	}
	return result
}

// publicProjectPaths preserves nil vs empty: the JSON contract distinguishes an
// absent list from an empty one.
func publicProjectPaths(projectDir string, values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = PublicProjectPath(projectDir, value)
	}
	return out
}
