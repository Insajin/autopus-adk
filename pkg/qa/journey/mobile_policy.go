package journey

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/mobile"
)

var sha256DigestRe = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

const mobilePolicyInvalidCode = "qa_journey_mobile_policy_invalid"

func validateMobilePolicy(pack Pack, projectDir string) error {
	switch pack.Adapter.ID {
	case "maestro-scripted":
		return validateMaestroPolicy(pack, projectDir)
	case "appium-mobile-explore":
		return validateAppiumPolicy(pack)
	default:
		return nil
	}
}

func validateMaestroPolicy(pack Pack, projectDir string) error {
	if err := validateProjectLocalMobilePath(pack.Mobile.FlowPath, projectDir); err != nil {
		return validationError(mobile.ReasonProjectLocalFlowRequired, err.Error())
	}
	if !strings.HasSuffix(strings.ToLower(pack.Mobile.FlowPath), ".yaml") &&
		!strings.HasSuffix(strings.ToLower(pack.Mobile.FlowPath), ".yml") {
		return validationError(mobilePolicyInvalidCode, "maestro flow must be YAML")
	}
	if err := validateOpaqueDeviceTarget(pack.Mobile.DeviceTarget, "maestro-scripted"); err != nil {
		return err
	}
	commandFlow, ok := maestroCommandFlowPath(pack.Command)
	if ok && normalizeMobileFlowPath(commandFlow) != normalizeMobileFlowPath(pack.Mobile.FlowPath) {
		return validationError(mobilePolicyInvalidCode, "maestro command flow must match mobile.flow_path")
	}
	return nil
}

func validateAppiumPolicy(pack Pack) error {
	if strings.EqualFold(strings.TrimSpace(pack.PassFailAuthority), "ai") {
		return validationError(mobilePolicyInvalidCode, "Appium mobile exploration cannot use AI pass/fail authority")
	}
	if err := validateOpaqueDeviceTarget(pack.Mobile.DeviceTarget, "appium-mobile-explore"); err != nil {
		return err
	}
	if !sha256DigestRe.MatchString(strings.TrimSpace(pack.Mobile.AppArtifactDigest)) {
		return validationError(mobilePolicyInvalidCode, "appium-mobile-explore requires app artifact digest")
	}
	if strings.TrimSpace(pack.Command.Timeout) == "" {
		return validationError(mobilePolicyInvalidCode, "appium-mobile-explore requires timeout")
	}
	if len(pack.Mobile.ForbiddenActions) == 0 {
		return validationError(mobilePolicyInvalidCode, "appium-mobile-explore requires forbidden actions")
	}
	if len(pack.Checks) == 0 {
		return validationError(mobilePolicyInvalidCode, "appium-mobile-explore requires deterministic checks")
	}
	return nil
}

func validateOpaqueDeviceTarget(ref, adapterID string) error {
	if !qaevidence.IsOpaqueMobileRef(ref) {
		return validationError(mobilePolicyInvalidCode, adapterID+" requires opaque device target")
	}
	return nil
}

func maestroCommandFlowPath(command Command) (string, bool) {
	argv := command.Argv
	if len(argv) == 0 && strings.TrimSpace(command.Run) != "" {
		argv = strings.Fields(command.Run)
	}
	if len(argv) < 3 {
		return "", false
	}
	for _, arg := range argv[2:] {
		if !strings.HasPrefix(arg, "-") {
			return arg, true
		}
	}
	return "", false
}

func normalizeMobileFlowPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: Maestro mobile flows must be project-local human-authored assets.
// @AX:REASON: Journey validation, readiness reporting, and evidence provenance rely on rejecting absolute paths and generated harness surfaces here.
func validateProjectLocalMobilePath(path, projectDir string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("project-local mobile flow path is required")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("project-local mobile flow path must be relative")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(clean, ".autopus/qa/mobile/") &&
		!strings.HasPrefix(clean, ".autopus/qa/journeys/") {
		return fmt.Errorf("project-local mobile flow path must live under .autopus/qa/mobile or .autopus/qa/journeys")
	}
	if strings.Contains(strings.ToLower(clean), "/.codex/") ||
		strings.HasPrefix(strings.ToLower(clean), ".codex/") {
		return fmt.Errorf("generated paths cannot satisfy project-local mobile flow readiness")
	}
	return nil
}
