package run

import "github.com/insajin/autopus-adk/pkg/qa/journey"

// applyMobileOracle delegates to the proven deterministic oracle. The scripted
// mobile lane passes only when the exit code matches and every declared
// assertion holds; there is no AI pass/fail authority.
func applyMobileOracle(projectDir string, pack journey.Pack, result *commandResult, check *IndexCheck) {
	applyOracle(projectDir, pack, result, check)
}
