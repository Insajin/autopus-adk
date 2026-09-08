package codex

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/insajin/autopus-adk/pkg/codexruntime"
)

const codexVersionTimeout = 5 * time.Second

// prepareCodexVersion reads the installed CLI release once per adapter. A
// failed probe is recorded as "probed, unknown" so generation never retries it
// per template and never silently assumes a release.
func (a *Adapter) prepareCodexVersion(ctx context.Context) {
	if a.codexVersionProbed {
		return
	}
	a.codexVersionProbed = true
	version, ok := codexruntime.ProbeVersion(ctx, cliBinary, codexVersionTimeout)
	if !ok {
		a.codexVersion = ""
		return
	}
	a.codexVersion = version
}

func (a *Adapter) agentConcurrencyNamespace() (
	codexruntime.AgentConcurrencyNamespace, codexruntime.AgentConcurrencyEvidence,
) {
	return codexruntime.AgentConcurrencyNamespaceFor(a.codexVersion)
}

// CodexAgentConcurrency is the spawned-worker ceiling the project requests.
// The coordinator session is not part of the count.
func (c codexRenderContext) CodexAgentConcurrency() int {
	return c.HarnessConfig.CodexAgentConcurrency()
}

// CodexAgentConcurrencyNote is the generated-config comment: it states the
// count semantics and the provenance of the namespace, which is either a
// verified release, an unverified one, or an unreadable version.
func (c codexRenderContext) CodexAgentConcurrencyNote() string {
	namespace, evidence := c.adapter.agentConcurrencyNamespace()
	return codexruntime.DescribeAgentConcurrencyNamespace(namespace, evidence)
}

// AgentConcurrencySetting is one spawned-agent ceiling assignment found in a
// Codex config file.
type AgentConcurrencySetting struct {
	Namespace string
	Key       string
	Value     int
	Malformed bool
}

// AgentConcurrencyInspection reports the spawned-agent ceiling a Codex config
// file carries. Namespace and Key name where the reported value came from,
// which is how a caller tells the documented key from its documented legacy
// alias or from the table an older Autopus wrote. Settings carries every
// ceiling in the file, because a second one is not noise: agents.max_threads
// is documented as an alias of agents.max_concurrent_threads_per_session, and
// which name a session applies when both are set is not documented at all.
type AgentConcurrencyInspection struct {
	Found     bool
	Value     int
	Namespace string
	Key       string
	Malformed bool
	// Settings lists every ceiling found, highest precedence first.
	Settings []AgentConcurrencySetting
}

// agentConcurrencySource is one place a ceiling can be spelled in a config
// file: a table plus the key inside it.
type agentConcurrencySource struct {
	namespace string
	key       string
}

// agentConcurrencySources lists every place a ceiling can sit, in reporting
// precedence: the documented key, then its documented legacy alias in the same
// table, then the undocumented table older Autopus releases wrote.
func agentConcurrencySources() []agentConcurrencySource {
	return []agentConcurrencySource{
		{namespace: string(codexruntime.AgentsNamespace), key: codexruntime.AgentConcurrencyKey},
		{
			namespace: string(codexruntime.AgentsNamespace),
			key:       codexruntime.LegacyAgentConcurrencyAliasKey,
		},
		{
			namespace: string(codexruntime.LegacyMultiAgentV2Namespace),
			key:       codexruntime.AgentConcurrencyKey,
		},
	}
}

// InspectAgentConcurrency reads the spawned-agent ceiling from a Codex config
// file. A missing file reports Found=false without an error, because "no
// project config" is a normal state rather than a diagnostic failure.
func InspectAgentConcurrency(path string) (AgentConcurrencyInspection, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return AgentConcurrencyInspection{}, nil
	}
	if err != nil {
		return AgentConcurrencyInspection{}, err
	}
	return inspectAgentConcurrencyContent(string(data)), nil
}

func inspectAgentConcurrencyContent(content string) AgentConcurrencyInspection {
	var inspection AgentConcurrencyInspection
	for _, source := range agentConcurrencySources() {
		raw, ok := sectionConfigValue(content, source.namespace, source.key)
		if !ok {
			continue
		}
		setting := AgentConcurrencySetting{Namespace: source.namespace, Key: source.key}
		if value, convErr := strconv.Atoi(raw); convErr == nil {
			setting.Value = value
		} else {
			setting.Malformed = true
		}
		inspection.Settings = append(inspection.Settings, setting)
		if inspection.Found {
			continue
		}
		inspection.Found = true
		inspection.Value = setting.Value
		inspection.Namespace = setting.Namespace
		inspection.Key = setting.Key
		inspection.Malformed = setting.Malformed
	}
	return inspection
}
