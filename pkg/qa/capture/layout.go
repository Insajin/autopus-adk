package capture

import "path/filepath"

// LocalCaptureDir derives a journey's local capture directory from the directory
// holding its published evidence manifest.
//
// This exists so the layout has exactly one definition. The runner allocates
// `<runDir>/_raw/<journey>/capture` and publishes the manifest to
// `<runDir>/<journey>/manifest.json`; a consumer that wants the raw media a
// published index references must be able to find it without the manifest
// recording an absolute local path, which would leak the user's filesystem into
// shareable evidence.
func LocalCaptureDir(manifestDir string) string {
	if manifestDir == "" {
		return ""
	}
	runDir := filepath.Dir(manifestDir)
	journey := filepath.Base(manifestDir)
	return filepath.Join(runDir, RawDirName, journey, DirName)
}
