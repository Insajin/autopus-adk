package scaffold

// captureProducerRootRel is the project-local home of the GUI capture producer.
//
// The producer is project-owned by design: the harness allocates a capture
// directory and declares the policy, but it never drives Playwright, so the code
// that fills the directory has to live in the repo the run executes from.
const captureProducerRootRel = ".autopus/qa/capture"

const captureProducerReason = "GUI capture contract requires a project-local Playwright producer"

// captureProducerStarters returns the capture producer assets emitted alongside a
// GUI or Playwright Journey Pack. They declare no lanes, so journey coverage never
// suppresses them, and they flow through the same non-overwrite path as every
// other starter: a project that already tuned its producer keeps its own version.
//
// The README is signal-dependent because it carries the gui-explore Journey Pack
// this project has to write itself, and that pack's origin and runner argv depend
// on the detected baseURL and package manager.
func captureProducerStarters(signals projectSignals) []starterFile {
	return []starterFile{{
		ID:      "gui-capture-fixture",
		RelPath: captureProducerRootRel + "/autopus-capture.fixture.cjs",
		Reason:  captureProducerReason,
		Body:    captureFixtureBody,
	}, {
		ID:      "gui-capture-reporter",
		RelPath: captureProducerRootRel + "/autopus-capture.reporter.cjs",
		Reason:  captureProducerReason,
		Body:    captureReporterBody,
	}, {
		ID:      "gui-capture-readme",
		RelPath: captureProducerRootRel + "/README.md",
		Reason:  captureProducerReason,
		Body:    captureReadmeBody(signals),
	}}
}
