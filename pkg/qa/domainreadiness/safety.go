package domainreadiness

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var shellMeta = regexp.MustCompile(`[;&|<>$` + "`" + `]|\|\||&&|\r|\n`)

func ValidateCatalogSource(path string) error {
	rel := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	denied := []string{
		".codex",
		".claude",
		".gemini",
		".opencode",
		".agents/skills",
		".agents/plugins",
		".agents/commands",
		".autopus/plugins",
		".autopus/qa/runs",
		".autopus/qa/cache",
		".autopus/qa/gui",
		".autopus/qa/feedback",
		".autopus/qa/evidence",
		".autopus/qa/releases",
		".autopus/brainstorms",
		".autopus/orchestra",
	}
	for _, prefix := range denied {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") || strings.Contains(rel, "/"+prefix+"/") {
			return fmt.Errorf("catalog source may not be generated surface %s", prefix)
		}
	}
	if rel == ".agents/hooks.json" || strings.HasSuffix(rel, "/.agents/hooks.json") {
		return fmt.Errorf("catalog source may not be generated surface .agents/hooks.json")
	}
	if strings.HasPrefix(filepath.Base(rel), ".autopus-") && strings.HasSuffix(rel, "-manifest.json") {
		return fmt.Errorf("catalog source may not be generated manifest")
	}
	return nil
}

func effectiveCommand(env SafeExecutionEnvironment) *CommandShape {
	if env.Command == nil {
		return nil
	}
	command := *env.Command
	if strings.TrimSpace(command.CWD) == "" {
		command.CWD = env.CWD
	}
	if strings.TrimSpace(command.Timeout) == "" {
		command.Timeout = env.Timeout
	}
	if len(command.EnvAllowlist) == 0 {
		command.EnvAllowlist = env.EnvAllowlist
	}
	return &command
}

func validateCommandShape(command CommandShape) []string {
	var findings []string
	cwd := strings.TrimSpace(command.CWD)
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) || cwd == ".." || strings.HasPrefix(filepath.Clean(cwd), ".."+string(filepath.Separator)) {
		findings = appendUniqueString(findings, "unsafe_cwd")
	}
	if !validTimeout(command.Timeout) {
		findings = appendUniqueString(findings, "unsafe_timeout")
	}
	for _, env := range command.EnvAllowlist {
		if unsafeEnvAllowlist(env) {
			findings = appendUniqueString(findings, "unsafe_env")
			break
		}
	}
	argv := command.Argv
	if len(argv) == 0 && strings.TrimSpace(command.Run) != "" {
		if shellMeta.MatchString(command.Run) {
			findings = appendUniqueString(findings, "unsafe_shell")
		}
		argv = strings.Fields(command.Run)
	}
	if containsUnsafeArgv(argv) {
		findings = appendUniqueString(findings, "unsafe_shell")
	}
	if len(argv) > 0 && !knownAdapterCommand(command.Adapter, argv) {
		findings = appendUniqueString(findings, "invented_command")
	}
	return findings
}

func classifyUnsafeAction(action string) UnsafeReason {
	normalized := actionFinding(action)
	if strings.Contains(normalized, "broad_scraping") || strings.Contains(normalized, "scrape_all") {
		return UnsafeReasonBroadScrapingNotAllowed
	}
	for _, fragment := range mutationActionFragments() {
		if strings.Contains(normalized, fragment) {
			return UnsafeReasonProductionMutationForbidden
		}
	}
	return ""
}

func mutationActionFragments() []string {
	return []string{
		"production_deploy",
		"prod_deploy",
		"production_rollback",
		"email_send",
		"send_email",
		"payment",
		"legal_commitment",
		"payroll",
		"bank_transfer",
		"publishing",
		"publish",
		"access_grant",
		"provider_write",
		"ads_write",
		"refund",
		"signature",
		"tax_filing",
		"invoice_send",
		"accounting_write",
	}
}

func actionFinding(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	action = strings.NewReplacer("-", "_", " ", "_", "/", "_", ":", "_").Replace(action)
	for strings.Contains(action, "__") {
		action = strings.ReplaceAll(action, "__", "_")
	}
	return strings.Trim(action, "_")
}

func validTimeout(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	duration, err := time.ParseDuration(value)
	return err == nil && duration > 0 && duration <= 30*time.Minute
}
