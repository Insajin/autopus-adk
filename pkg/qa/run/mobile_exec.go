package run

import (
	"context"
	"os"
	"path/filepath"
	"time"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// @AX:NOTE: [AUTO] magic constant — scripted mobile lane default timeout (120s); override via pack.Command.Timeout
const mobileDefaultTimeout = 120 * time.Second

// executeMobilePack runs the scripted Maestro lane for a single mobile pack. It
// resolves the device, runs the flow through the shared engine, applies the
// deterministic oracle, writes opaque mobile artifacts, and emits a V2 manifest.
// The concrete device handle never enters the pack copy or published evidence.
func executeMobilePack(opts Options, pack journey.Pack, rawRoot, runDir string) (AdapterResult, string, []IndexCheck) {
	runner := opts.deviceRunner
	if runner == nil {
		runner = realMobileDeviceRunner{}
	}
	artifactDir := filepath.Join(rawRoot, safeSegment(pack.ID))
	_ = os.MkdirAll(artifactDir, 0o755)
	ctx, cancel := context.WithTimeout(context.Background(), mobilePackTimeout(pack))
	defer cancel()

	check := IndexCheck{ID: firstCheckID(pack), JourneyID: pack.ID, Adapter: pack.Adapter.ID, Expected: "exit_code=0"}
	devCtx, gap := prepareMobileDevice(ctx, opts, pack, runner)
	if gap != nil {
		check.Status = "skipped"
		return AdapterResult{Adapter: pack.Adapter.ID, JourneyID: pack.ID, Status: "skipped", SetupGap: gap}, "", []IndexCheck{check}
	}

	cmdResult := runner.RunFlow(ctx, mobileFlowRequest{
		ProjectDir:  opts.ProjectDir,
		Pack:        pack,
		Handle:      devCtx.Handle,
		ArtifactDir: artifactDir,
	})
	applyMobileOracle(opts.ProjectDir, pack, &cmdResult, &check)
	// Scrub the concrete device handle from published logs/text before the
	// manifest is built; the flow tooling echoes it into stdout (INV-Q8-002).
	redactMobileHandle(&cmdResult, &check, devCtx.Handle)

	packCopy := writeMobileArtifacts(artifactDir, pack, devCtx)
	manifest := buildManifest(opts, packCopy, cmdResult, []IndexCheck{check})
	manifestPath, err := qaevidence.WriteFinalManifest(manifest, manifestOutputDir(runDir, pack.ID))
	if err != nil {
		check.Status = "blocked"
		check.FailureSummary = err.Error()
		return AdapterResult{
			Adapter:               pack.Adapter.ID,
			JourneyID:             pack.ID,
			Status:                "blocked",
			RepairPromptAvailable: false,
			FailureSummary:        err.Error(),
		}, "", []IndexCheck{check}
	}
	return AdapterResult{
		Adapter:               pack.Adapter.ID,
		JourneyID:             pack.ID,
		Status:                cmdResult.Status,
		QAMESHManifestPath:    manifestPath,
		RepairPromptAvailable: cmdResult.Status != "passed",
		FailureSummary:        cmdResult.FailureSummary,
	}, manifestPath, []IndexCheck{check}
}

func mobilePackTimeout(pack journey.Pack) time.Duration {
	if parsed, err := time.ParseDuration(pack.Command.Timeout); err == nil && parsed > 0 {
		return parsed
	}
	return mobileDefaultTimeout
}
