package release

import qarun "github.com/insajin/autopus-adk/pkg/qa/run"

// publicOutputPaths rewrites the published roots as project-root-relative refs.
//
// These are the same fields that came back empty from an absolute
// --project-dir: they were absolute, private-path redaction matched them, and
// what survived was "". Publishing them relative means there is nothing for
// redaction to match, so the release index keeps a usable evidence chain
// regardless of how --project-dir was spelled.
func publicOutputPaths(projectDir string, paths OutputPaths) OutputPaths {
	paths.ReleaseIndexPreviewPath = qarun.PublicProjectPath(projectDir, paths.ReleaseIndexPreviewPath)
	paths.ReleaseIndexPath = qarun.PublicProjectPath(projectDir, paths.ReleaseIndexPath)
	paths.RunIndexRoot = qarun.PublicProjectPath(projectDir, paths.RunIndexRoot)
	paths.EvidenceRoot = qarun.PublicProjectPath(projectDir, paths.EvidenceRoot)
	paths.FeedbackRoot = qarun.PublicProjectPath(projectDir, paths.FeedbackRoot)
	return paths
}
