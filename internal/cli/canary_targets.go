package cli

import "path/filepath"

// canaryBuildTargets derives the build health checks from the project under
// test. It used to return a fixed list of Autopus monorepo directories, so any
// other project failed every build check before a process even started - and
// `auto qa init` wires `auto canary` into the canary-explicit lane for every
// project, which made that lane unsatisfiable outside the monorepo.
//
// The returned skips explain every detected stack that contributes no command,
// because a silently dropped stack reads as "canary checked this" when it did
// not.
func canaryBuildTargets(projectDir string) ([]canaryCommandTarget, []canarySkippedCheck) {
	var (
		targets []canaryCommandTarget
		skips   []canarySkippedCheck
	)
	for _, unit := range canaryBuildUnits(projectDir) {
		unitTargets, unitSkips := canaryUnitTargets(unit)
		targets = append(targets, unitTargets...)
		skips = append(skips, unitSkips...)
	}
	return targets, skips
}

func canaryUnitTargets(unit canaryBuildUnit) ([]canaryCommandTarget, []canarySkippedCheck) {
	var (
		targets []canaryCommandTarget
		skips   []canarySkippedCheck
	)
	// canaryStackOrder rather than unit.Stacks: ordering is an evidence contract,
	// and a hand-assembled unit must render the same sequence as a detected one.
	for _, stack := range canaryStackOrder {
		if !canaryUnitHasStack(unit, stack) {
			continue
		}
		stackTargets, reason := canaryStackTargets(unit, stack)
		targets = append(targets, stackTargets...)
		if reason != "" {
			skips = append(skips, canarySkippedCheck{Area: canaryTargetID(unit, stack), Reason: reason})
		}
	}
	return targets, skips
}

func canaryStackTargets(unit canaryBuildUnit, stack string) ([]canaryCommandTarget, string) {
	switch stack {
	case "go":
		return []canaryCommandTarget{{
			ID:      canaryTargetID(unit, "go"),
			Dir:     unit.Dir,
			Command: "go build ./...",
			Args:    []string{"go", "build", "./..."},
		}}, ""
	case "node":
		return canaryNodeTargets(unit)
	case "rust":
		return []canaryCommandTarget{{
			ID:      canaryTargetID(unit, "rust"),
			Dir:     unit.Dir,
			Command: "cargo check",
			Args:    []string{"cargo", "check"},
		}}, ""
	case "python":
		// Python has no universal build step. Compiling sources would assert a
		// health property the project never declared, so state the gap instead.
		return nil, "Python stack declares no build command"
	default:
		return nil, "unsupported stack " + stack
	}
}

func canaryNodeTargets(unit canaryBuildUnit) ([]canaryCommandTarget, string) {
	var targets []canaryCommandTarget
	reason := ""
	if _, ok := canaryPackageScripts(unit.Dir)["build"]; ok {
		pm := canaryPackageManager(unit.Dir)
		targets = append(targets, canaryCommandTarget{
			ID:      canaryTargetID(unit, "node"),
			Dir:     unit.Dir,
			Command: pm + " run build",
			Args:    []string{pm, "run", "build"},
		})
	} else {
		reason = "package.json declares no build script"
	}
	// A Tauri shell ships a Rust crate under src-tauri that the node build never
	// compiles; without this the desktop half of the app is unchecked.
	if canaryFileExists(unit.Dir, "src-tauri/Cargo.toml") {
		targets = append(targets, canaryCommandTarget{
			ID:      canaryTargetID(unit, "tauri"),
			Dir:     unit.Dir,
			Command: "cargo check --manifest-path src-tauri/Cargo.toml",
			Args:    []string{"cargo", "check", "--manifest-path", filepath.FromSlash("src-tauri/Cargo.toml")},
		})
	}
	return targets, reason
}

func canaryTargetID(unit canaryBuildUnit, stack string) string {
	return "build:" + unit.Rel + ":" + stack
}
