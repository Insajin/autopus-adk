package scaffold

// fastLaneStackOrder fixes the emission order of fast-lane starters. Order is
// pinned rather than derived from directory reads so repeated `auto qa init` runs
// on one repo declare the same packs in the same sequence; the release gate diffs
// declared packs, and a shuffling set would read as configuration drift.
var fastLaneStackOrder = []string{"go", "node", "python", "rust"}

// dominantStackOrder resolves projectSignals.Stack, the single-stack view kept for
// consumers that must pick one answer (QA target scoring, CI workflow rendering).
// Node precedes Go on purpose: it reproduces exactly the winner the previous
// last-write-wins detection produced for every stack combination, so adding fast
// lanes for the other stacks present cannot shift target scores or rendered
// workflow steps. Changing this order is a behaviour change, not a cleanup.
var dominantStackOrder = []string{"node", "go", "python", "rust"}

// detectStacks reports every stack whose marker files are present. Earlier
// versions stopped at the first (really: the last) match, which is why a repo
// could own a green `go test ./...` and still declare no fast lane.
func detectStacks(projectDir string) []string {
	stacks := make([]string, 0, len(fastLaneStackOrder))
	if exists(projectDir, "go.mod") {
		stacks = append(stacks, "go")
	}
	if exists(projectDir, "package.json") {
		stacks = append(stacks, "node")
	}
	if exists(projectDir, "pyproject.toml") || exists(projectDir, "requirements.txt") || exists(projectDir, "pytest.ini") {
		stacks = append(stacks, "python")
	}
	if exists(projectDir, "Cargo.toml") {
		stacks = append(stacks, "rust")
	}
	return stacks
}

func dominantStack(stacks []string) string {
	for _, candidate := range dominantStackOrder {
		if containsStack(stacks, candidate) {
			return candidate
		}
	}
	return ""
}

func containsStack(stacks []string, want string) bool {
	for _, stack := range stacks {
		if stack == want {
			return true
		}
	}
	return false
}

// fastStarters emits one fast-lane starter per detected stack.
//
// A polyglot repo used to get at most one: detection kept a single stack and
// package.json overwrote go.mod, so a Node package with no runnable test signal
// (no `test` script, no vitest, no jest, no `build` script) contributed nothing
// and silently suppressed the Go fast lane too. `fast` is a `must` lane in the
// default prelaunch profile, so the repo landed on `setup_gap:
// missing-journey-pack` and `auto qa release` returned gate `blocked` despite a
// green `go test ./...`. `auto qa plan` hid the defect because it scores compiled
// candidates, while the release gate requires declared packs.
//
// Multiple packs on one lane are supported: run.Execute iterates every selected
// pack, so no cross-pack dedup belongs here.
func fastStarters(signals projectSignals) []starterFile {
	starters := make([]starterFile, 0, len(signals.Stacks))
	// Iterate the canonical order instead of signals.Stacks so ordering holds even
	// when a caller assembles signals by hand.
	for _, stack := range fastLaneStackOrder {
		if !containsStack(signals.Stacks, stack) {
			continue
		}
		if starter, ok := stackFastStarter(stack, signals); ok {
			starters = append(starters, starter)
		}
	}
	return starters
}

func stackFastStarter(stack string, signals projectSignals) (starterFile, bool) {
	switch stack {
	case "go":
		return journeyStarter("go-fast", "Go fast test lane", "cli", []string{"fast"}, "go-test", []string{"go", "test", "./..."}, "Go module detected"), true
	case "node":
		return nodeFastStarter(signals)
	case "python":
		return journeyStarter("python-fast", "Python fast test lane", "cli", []string{"fast"}, "pytest", []string{"python", "-m", "pytest"}, "Python test signals detected"), true
	case "rust":
		return journeyStarter("rust-fast", "Rust fast test lane", "cli", []string{"fast"}, "cargo-test", []string{"cargo", "test"}, "Cargo project detected"), true
	default:
		return starterFile{}, false
	}
}

// nodeFastStarter keeps its internal precedence (test script, vitest, jest, build
// script). Returning false now only drops the Node lane; the other detected
// stacks still contribute their own starters.
func nodeFastStarter(signals projectSignals) (starterFile, bool) {
	pm := nodeCommand(signals.PackageManager)
	if hasScript(signals.Package, "test") {
		return journeyStarter("node-fast", "Node fast test lane", "package", []string{"fast"}, "node-script", []string{pm, "test"}, "package.json test script detected"), true
	}
	if hasFileSignal("vitest", signals.Package) {
		return journeyStarter("vitest-fast", "Vitest fast test lane", "frontend", []string{"fast"}, "vitest", jsRunnerArgv(pm, "vitest", "run"), "Vitest signals detected"), true
	}
	if hasDependency(signals.Package, "jest") {
		return journeyStarter("jest-fast", "Jest fast test lane", "frontend", []string{"fast"}, "jest", jsRunnerArgv(pm, "jest"), "Jest dependency detected"), true
	}
	if hasScript(signals.Package, "build") {
		return journeyStarter("node-build-fast", "Node build fast lane", "package", []string{"fast"}, "node-script", []string{pm, "run", "build"}, "package.json build script detected"), true
	}
	return starterFile{}, false
}
