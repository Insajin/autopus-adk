package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/codexruntime"
	"github.com/insajin/autopus-adk/pkg/config"
)

const doctorCodexAgentConcurrencyCheckID = "doctor.codex.agents.concurrency"

// codexAgentConcurrencyUnknown is the only honest answer for both the loaded
// and the effective limit: see codexruntime.LoadedAgentConcurrencyUnknownReason
// and codexruntime.EffectiveAgentConcurrencyUnknownReason. What a config file
// carries is reported separately, as on-disk configuration.
const codexAgentConcurrencyUnknown = "unknown"

const (
	codexAgentConcurrencyFromProject = "autopus.yaml"
	codexAgentConcurrencyFromDefault = "harness default"
)

type codexAgentConcurrencyDiagnosis struct {
	status      string
	detail      string
	fields      map[string]string
	warningCode string
}

// checkCodexAgentConcurrencyText renders the advisory concurrency report. It
// never fails harness health: a drifted ceiling is worth naming, but it does
// not make the harness unusable.
func checkCodexAgentConcurrencyText(w io.Writer, dir string, cfg *config.HarnessConfig) {
	tui.SectionHeader(w, "Codex Agent Concurrency")
	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)
	switch diagnosis.status {
	case "warn":
		tui.SKIP(w, diagnosis.detail)
	case "skip":
		tui.Info(w, diagnosis.detail)
	default:
		tui.OK(w, diagnosis.detail)
	}
	if diagnosis.status != "skip" {
		tui.Bullet(w, codexruntime.LoadedAgentConcurrencyUnknownReason)
		tui.Bullet(w, codexruntime.EffectiveAgentConcurrencyUnknownReason)
	}
}

func (r *doctorJSONReport) collectCodexAgentConcurrencyCheck(dir string, cfg *config.HarnessConfig) {
	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)
	severity := "info"
	if diagnosis.status == "warn" {
		severity = "warning"
		r.status = jsonStatusWarn
		r.warnings = append(r.warnings, jsonMessage{
			Code:    diagnosis.warningCode,
			Message: diagnosis.detail,
		})
	}
	r.checks = append(r.checks, jsonCheck{
		ID:       doctorCodexAgentConcurrencyCheckID,
		Severity: severity,
		Status:   diagnosis.status,
		Detail:   diagnosis.detail,
		Fields:   diagnosis.fields,
	})
}

func diagnoseCodexAgentConcurrency(dir string, cfg *config.HarnessConfig) codexAgentConcurrencyDiagnosis {
	if cfg == nil || !containsPlatform(cfg.Platforms, "codex") {
		return codexAgentConcurrencyDiagnosis{
			status: "skip",
			detail: "Codex agent concurrency check is not applicable",
		}
	}
	requested := requestedCodexAgentConcurrency(cfg)
	onDisk, err := loadCodexAgentConcurrency(dir)
	if err != nil {
		return codexAgentConcurrencyDiagnosis{
			status:      "warn",
			detail:      fmt.Sprintf("Codex agent concurrency inspection failed: %v", err),
			fields:      codexAgentConcurrencyFields(requested, onDisk),
			warningCode: "codex_agent_concurrency_inspection_failed",
		}
	}
	diagnosis := codexAgentConcurrencyDiagnosis{
		status: "pass",
		fields: codexAgentConcurrencyFields(requested, onDisk),
		detail: fmt.Sprintf(
			"requested %d spawned workers (excluding the coordinator; from %s); on disk %s; loaded %s; effective %s",
			requested.value, requested.source, onDisk.describe(),
			codexAgentConcurrencyUnknown, codexAgentConcurrencyUnknown,
		),
	}
	switch {
	case onDisk.malformed:
		diagnosis.status = "warn"
		diagnosis.warningCode = "codex_agent_concurrency_malformed"
		diagnosis.detail = fmt.Sprintf(
			"%s carries a non-numeric %s; run 'auto update'",
			onDisk.origin, onDisk.key,
		)
	case len(onDisk.conflicts) > 0:
		diagnosis.status = "warn"
		diagnosis.warningCode = "codex_agent_concurrency_conflict"
		diagnosis.detail = fmt.Sprintf(
			"on disk %s disagrees with %s; Codex does not document which one a session applies, "+
				"so the ceiling stays unknown until one is removed",
			onDisk.describe(), onDisk.describeConflicts(),
		)
	case !onDisk.found:
		diagnosis.status = "warn"
		diagnosis.warningCode = "codex_agent_concurrency_unset"
		diagnosis.detail = fmt.Sprintf(
			"requested %d spawned workers but no Codex config sets %s; run 'auto update'",
			requested.value, codexruntime.AgentConcurrencyKey,
		)
	case onDisk.value != requested.value:
		diagnosis.status = "warn"
		diagnosis.warningCode = "codex_agent_concurrency_drift"
		diagnosis.detail = fmt.Sprintf(
			"requested %d spawned workers but %s carries %d on disk; run 'auto update'",
			requested.value, onDisk.origin, onDisk.value,
		)
	}
	return diagnosis
}

// codexAgentConcurrencyRequest is the count Autopus asks for and where that
// number came from. It is a request, never an observation: autopus.yaml is the
// only place a project declares it, and an unset value resolves to the harness
// default rather than to whatever a config file happens to carry.
type codexAgentConcurrencyRequest struct {
	value  int
	source string
}

func requestedCodexAgentConcurrency(cfg *config.HarnessConfig) codexAgentConcurrencyRequest {
	request := codexAgentConcurrencyRequest{
		value:  cfg.CodexAgentConcurrency(),
		source: codexAgentConcurrencyFromDefault,
	}
	if cfg.Codex.Agents.MaxConcurrentThreads != 0 {
		request.source = codexAgentConcurrencyFromProject
	}
	return request
}

func codexAgentConcurrencyFields(
	requested codexAgentConcurrencyRequest,
	onDisk codexAgentConcurrencySource,
) map[string]string {
	fields := map[string]string{
		"requested":        strconv.Itoa(requested.value),
		"requested_source": requested.source,
		"on_disk":          onDisk.valueField(),
		"on_disk_origin":   onDisk.origin,
		"loaded":           codexAgentConcurrencyUnknown,
		"loaded_reason":    codexruntime.LoadedAgentConcurrencyUnknownReason,
		"effective":        codexAgentConcurrencyUnknown,
		"effective_reason": codexruntime.EffectiveAgentConcurrencyUnknownReason,
	}
	if onDisk.namespace != "" {
		fields["on_disk_namespace"] = onDisk.namespace
		fields["on_disk_key"] = onDisk.key
	}
	if len(onDisk.conflicts) > 0 {
		fields["on_disk_conflicts"] = onDisk.describeConflicts()
	}
	return fields
}

// codexAgentConcurrencySource is the ceiling a config file on disk carries.
// It is not session state: nothing here says a running Codex parsed it.
type codexAgentConcurrencySource struct {
	found     bool
	malformed bool
	value     int
	origin    string
	namespace string
	key       string
	conflicts []codexAgentConcurrencySetting
}

// codexAgentConcurrencySetting is a second ceiling that disagrees with the
// reported one, whether it sits in another layer or under the documented
// agents.max_threads alias in the same table.
type codexAgentConcurrencySetting struct {
	origin    string
	namespace string
	key       string
	value     int
	malformed bool
}

func (s codexAgentConcurrencySource) valueField() string {
	if !s.found || s.malformed {
		return codexAgentConcurrencyUnknown
	}
	return strconv.Itoa(s.value)
}

func (s codexAgentConcurrencySource) describe() string {
	if s.malformed {
		return fmt.Sprintf("a malformed value in %s [%s].%s", s.origin, s.namespace, s.key)
	}
	if !s.found {
		return fmt.Sprintf("nothing (%s)", s.origin)
	}
	return fmt.Sprintf("%d from %s [%s].%s", s.value, s.origin, s.namespace, s.key)
}

func (s codexAgentConcurrencySource) describeConflicts() string {
	parts := make([]string, 0, len(s.conflicts))
	for _, conflict := range s.conflicts {
		value := codexAgentConcurrencyUnknown
		if !conflict.malformed {
			value = strconv.Itoa(conflict.value)
		}
		parts = append(parts, fmt.Sprintf("%s [%s].%s = %s",
			conflict.origin, conflict.namespace, conflict.key, value))
	}
	return strings.Join(parts, "; ")
}

// loadCodexAgentConcurrency collects every spawned-agent ceiling on disk, in
// the order Codex layers config: the project file first, then the user home
// file. The first one found is reported; any other value found anywhere is a
// conflict rather than a silently discarded setting, because the documented
// agents.max_threads alias and the table older Autopus wrote can both still be
// parsed and Codex does not document which one wins. Nothing setting it means
// Codex picks its own default, which the harness does not claim to know.
func loadCodexAgentConcurrency(dir string) (codexAgentConcurrencySource, error) {
	candidates := []struct {
		path   string
		origin string
	}{
		{filepath.Join(dir, ".codex", "config.toml"), "project .codex/config.toml"},
		{filepath.Join(codexHomeDir(), "config.toml"), "user " + filepath.Join(codexHomeDir(), "config.toml")},
	}
	source := codexAgentConcurrencySource{origin: "Codex built-in default"}
	for _, candidate := range candidates {
		inspection, err := codex.InspectAgentConcurrency(candidate.path)
		if err != nil {
			return codexAgentConcurrencySource{origin: candidate.origin}, err
		}
		for _, setting := range inspection.Settings {
			if !source.found {
				source = codexAgentConcurrencySource{
					found: true, malformed: setting.Malformed, value: setting.Value,
					origin: candidate.origin, namespace: setting.Namespace, key: setting.Key,
				}
				continue
			}
			if !setting.Malformed && !source.malformed && setting.Value == source.value {
				continue
			}
			source.conflicts = append(source.conflicts, codexAgentConcurrencySetting{
				origin: candidate.origin, namespace: setting.Namespace,
				key: setting.Key, value: setting.Value, malformed: setting.Malformed,
			})
		}
	}
	return source, nil
}

// codexHomeDir mirrors the CLI's own resolution: CODEX_HOME wins, otherwise
// ~/.codex.
func codexHomeDir() string {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return home
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(userHome, ".codex")
}
