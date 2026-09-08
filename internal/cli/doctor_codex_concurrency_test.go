package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func codexConcurrencyFixture(t *testing.T, requested int) (string, *config.HarnessConfig) {
	t.Helper()
	dir := t.TempDir()
	// A pinned CODEX_HOME keeps the developer's own ~/.codex out of the
	// resolution order, so an empty home means "Codex built-in default".
	t.Setenv("CODEX_HOME", filepath.Join(dir, "codex-home"))
	cfg := config.DefaultFullConfig("concurrency")
	cfg.Platforms = []string{"codex"}
	cfg.Codex.Agents.MaxConcurrentThreads = requested
	return dir, cfg
}

func writeCodexConcurrencyConfig(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

// A parsed config file is configuration on disk, not the state of a session.
// The check must keep the requested count, the on-disk value, the loaded value
// and the effective value apart, and answer "unknown" for the two it cannot
// observe instead of promoting the file's number into either.
func TestDiagnoseCodexAgentConcurrency_SeparatesOnDiskFromLoadedAndEffective(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_concurrent_threads_per_session = 8\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "pass", diagnosis.status)
	assert.Equal(t, "8", diagnosis.fields["requested"])
	assert.Equal(t, "autopus.yaml", diagnosis.fields["requested_source"])
	assert.Equal(t, "8", diagnosis.fields["on_disk"])
	assert.Equal(t, "project .codex/config.toml", diagnosis.fields["on_disk_origin"])
	assert.Equal(t, "agents", diagnosis.fields["on_disk_namespace"])
	assert.Equal(t, "max_concurrent_threads_per_session", diagnosis.fields["on_disk_key"])
	assert.Equal(t, "unknown", diagnosis.fields["loaded"])
	assert.Contains(t, diagnosis.fields["loaded_reason"], "codex doctor --json")
	assert.Equal(t, "unknown", diagnosis.fields["effective"])
	assert.Contains(t, diagnosis.fields["effective_reason"], "codex config get")
	assert.Contains(t, diagnosis.fields["effective_reason"], "new Codex session")
	assert.Contains(t, diagnosis.detail, "on disk 8")
	assert.Contains(t, diagnosis.detail, "loaded unknown")
}

// An unset codex.agents.max_concurrent_threads resolves to the harness
// default, and the report must say so: a reader otherwise takes the number for
// a value the project chose.
func TestDiagnoseCodexAgentConcurrency_NamesTheRequestProvenance(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 0)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_concurrent_threads_per_session = 4\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "pass", diagnosis.status)
	assert.Equal(t, "4", diagnosis.fields["requested"])
	assert.Equal(t, "harness default", diagnosis.fields["requested_source"])
}

func TestDiagnoseCodexAgentConcurrency_FallsBackToUserHomeThenCodexDefault(t *testing.T) {
	t.Run("user home", func(t *testing.T) {
		dir, cfg := codexConcurrencyFixture(t, 4)
		writeCodexConcurrencyConfig(t, filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"),
			"[agents]\nmax_concurrent_threads_per_session = 4\n")

		diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

		assert.Equal(t, "pass", diagnosis.status)
		assert.Equal(t, "4", diagnosis.fields["on_disk"])
		assert.Contains(t, diagnosis.fields["on_disk_origin"], "user ")
	})

	t.Run("nothing sets it", func(t *testing.T) {
		dir, cfg := codexConcurrencyFixture(t, 4)

		diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

		assert.Equal(t, "warn", diagnosis.status)
		assert.Equal(t, "codex_agent_concurrency_unset", diagnosis.warningCode)
		assert.Equal(t, "unknown", diagnosis.fields["on_disk"])
		assert.Equal(t, "Codex built-in default", diagnosis.fields["on_disk_origin"])
		assert.Contains(t, diagnosis.detail, "auto update")
	})
}

// The documented agents.max_threads alias configures the same ceiling, so a
// file setting only the alias is not unset. Reporting it as unset would send
// the user to 'auto update' for a setting that is already there.
func TestDiagnoseCodexAgentConcurrency_ReadsTheDocumentedLegacyAlias(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 6)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_threads = 6\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "pass", diagnosis.status)
	assert.Equal(t, "6", diagnosis.fields["on_disk"])
	assert.Equal(t, "max_threads", diagnosis.fields["on_disk_key"])
}

// Codex documents max_threads as an alias but never says which name wins, so
// two disagreeing ceilings leave nothing to report as the on-disk value. The
// check must name the conflict instead of silently picking one.
func TestDiagnoseCodexAgentConcurrency_WarnsWhenTwoCeilingsDisagree(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_concurrent_threads_per_session = 8\nmax_threads = 6\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "warn", diagnosis.status)
	assert.Equal(t, "codex_agent_concurrency_conflict", diagnosis.warningCode)
	assert.Contains(t, diagnosis.fields["on_disk_conflicts"], "[agents].max_threads = 6")
	assert.Contains(t, diagnosis.detail, "does not document which one")
}

// A ceiling in the user home file that disagrees with the project file is the
// same conflict across two layers: the project value is reported, and the
// other one is named rather than dropped.
func TestDiagnoseCodexAgentConcurrency_WarnsWhenAnotherLayerDisagrees(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_concurrent_threads_per_session = 8\n")
	writeCodexConcurrencyConfig(t, filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"),
		"[agents]\nmax_threads = 12\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "warn", diagnosis.status)
	assert.Equal(t, "codex_agent_concurrency_conflict", diagnosis.warningCode)
	assert.Equal(t, "8", diagnosis.fields["on_disk"])
	assert.Contains(t, diagnosis.fields["on_disk_conflicts"], "max_threads = 12")
}

// Drift is the requested count disagreeing with the file, which is a real
// finding: the harness asked for eight workers and the file the CLI reads
// still says four.
func TestDiagnoseCodexAgentConcurrency_WarnsOnDriftFromRequested(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[features.multi_agent_v2]\nmax_concurrent_threads_per_session = 4\n")

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "warn", diagnosis.status)
	assert.Equal(t, "codex_agent_concurrency_drift", diagnosis.warningCode)
	assert.Equal(t, "8", diagnosis.fields["requested"])
	assert.Equal(t, "4", diagnosis.fields["on_disk"])
	assert.Equal(t, "features.multi_agent_v2", diagnosis.fields["on_disk_namespace"])
	assert.Equal(t, "unknown", diagnosis.fields["effective"])
}

func TestDiagnoseCodexAgentConcurrency_SkipsWithoutCodexPlatform(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	cfg.Platforms = []string{"claude-code"}

	diagnosis := diagnoseCodexAgentConcurrency(dir, cfg)

	assert.Equal(t, "skip", diagnosis.status)
	assert.Contains(t, diagnosis.detail, "not applicable")
}

// Concurrency drift is advisory: it must warn without failing harness health,
// and the JSON report must keep the unknown effective limit so a consumer
// cannot read the on-disk value as observed capacity.
func TestCollectCodexAgentConcurrencyCheck_WarnsWithoutFailingHealth(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 8)
	writeCodexConcurrencyConfig(t, filepath.Join(dir, ".codex", "config.toml"),
		"[agents]\nmax_concurrent_threads_per_session = 2\n")

	report := doctorJSONReport{status: jsonStatusOK}
	report.collectCodexAgentConcurrencyCheck(dir, cfg)

	require.Len(t, report.checks, 1)
	check := report.checks[0]
	assert.Equal(t, doctorCodexAgentConcurrencyCheckID, check.ID)
	assert.Equal(t, "warning", check.Severity)
	assert.Equal(t, "unknown", check.Fields["effective"])
	assert.Equal(t, "unknown", check.Fields["loaded"])
	assert.Equal(t, "2", check.Fields["on_disk"])
	assert.Equal(t, jsonStatusWarn, report.status)
	require.Len(t, report.warnings, 1)
	assert.Equal(t, "codex_agent_concurrency_drift", report.warnings[0].Code)

	var out bytes.Buffer
	checkCodexAgentConcurrencyText(&out, dir, cfg)
	assert.Contains(t, out.String(), "Codex Agent Concurrency")
	assert.Contains(t, out.String(), "not observed session state")
	assert.Contains(t, out.String(), "new Codex session")
}

func TestCheckCodexAgentConcurrencyText_SkipStaysQuietAboutProbing(t *testing.T) {
	dir, cfg := codexConcurrencyFixture(t, 4)
	cfg.Platforms = []string{"claude-code"}

	var out bytes.Buffer
	checkCodexAgentConcurrencyText(&out, dir, cfg)

	assert.Contains(t, out.String(), "not applicable")
	assert.NotContains(t, out.String(), "new Codex session")
}
