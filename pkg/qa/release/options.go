package release

import (
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: default release paths stay under project .autopus QA roots to avoid generated surface writes.
func normalizeOptions(opts Options) Options {
	if opts.ProjectDir == "" {
		opts.ProjectDir = "."
	}
	if opts.Profile == "" {
		opts.Profile = "prelaunch"
	}
	if opts.Output == "" {
		opts.Output = filepath.Join(opts.ProjectDir, ".autopus", "qa", "releases")
	}
	if opts.RunOutputRoot == "" {
		opts.RunOutputRoot = filepath.Join(opts.ProjectDir, ".autopus", "qa", "runs")
	}
	if opts.Command == "" {
		opts.Command = "auto qa release"
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.NewID == nil {
		opts.NewID = func() string { return "release-" + uuid.NewString() }
	}
	return opts
}
