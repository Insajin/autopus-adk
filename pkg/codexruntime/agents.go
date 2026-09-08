package codexruntime

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// AgentConcurrencyKey is the config.toml key the Codex configuration reference
// documents for the spawned-agent ceiling: "Maximum number of spawned-agent
// threads that can be open concurrently, excluding the primary thread."
const AgentConcurrencyKey = "max_concurrent_threads_per_session"

// LegacyAgentConcurrencyAliasKey is the second name of that same setting. The
// configuration reference lists agents.max_threads as "Legacy alias for
// agents.max_concurrent_threads_per_session", and codex-cli 0.153.4 still
// parses it, so a config file can carry the ceiling under either name. Which
// one a session applies when both are set is not documented, so a file
// carrying both has no readable ceiling.
const LegacyAgentConcurrencyAliasKey = "max_threads"

// AgentConcurrencyNamespace is the config.toml table carrying the ceiling.
type AgentConcurrencyNamespace string

const (
	// AgentsNamespace is the only table the configuration reference documents
	// for this setting, so it is the only table Autopus writes.
	AgentsNamespace AgentConcurrencyNamespace = "agents"
	// LegacyMultiAgentV2Namespace is a table older Autopus releases wrote the
	// ceiling into. The current reference does not document it at all, while
	// codex-cli 0.153.4 still parses it, so an existing file must be read and
	// migrated from it - and no file may be generated with it.
	LegacyMultiAgentV2Namespace AgentConcurrencyNamespace = "features.multi_agent_v2"
)

// AgentConcurrencyEvidence records how a namespace decision was reached so a
// generated comment and a diagnostic can state provenance instead of
// presenting an assumption as a detected fact.
type AgentConcurrencyEvidence string

const (
	// AgentConcurrencyVerifiedRelease means this exact release was observed
	// accepting AgentConcurrencyKey under AgentsNamespace.
	AgentConcurrencyVerifiedRelease AgentConcurrencyEvidence = "verified_release"
	// AgentConcurrencyAssumedDocumented means the version parsed but was never
	// observed, so the documented table is an assumption about that release.
	AgentConcurrencyAssumedDocumented AgentConcurrencyEvidence = "assumed_documented"
	// AgentConcurrencyUnknownVersion means `codex --version` was unreadable, so
	// not even the release behind the assumption is known.
	AgentConcurrencyUnknownVersion AgentConcurrencyEvidence = "unknown_version"
)

// verifiedAgentConcurrencyReleases lists the releases whose acceptance of
// AgentConcurrencyKey under AgentsNamespace was observed directly, keyed by the
// exact release. A version string cannot prove what a neighbouring release
// parses, so an entry never widens into a floor, a ceiling, or a range: every
// unlisted release is an assumption and says so.
//
// 0.153.4: with CODEX_HOME pinned to a scratch home, `codex doctor --json`
// reports config.load "config loaded" for `[agents]
// max_concurrent_threads_per_session = 3` and "config could not be loaded" for
// an unknown key in that same table. The table therefore rejects unknown
// fields, which makes the first result proof that the key is recognised. That
// is the whole of the observation: it says nothing about the ceiling a running
// session applies (see EffectiveAgentConcurrencyUnknownReason), and the same
// probe shows the undocumented features.multi_agent_v2 table still parsing the
// key on this release, so "recognised" is not "sole namespace" either.
//
// The table is a recorded observation rather than a probe on the generation
// path because that observation is not cheap or side-effect free: `codex
// doctor --json` also runs provider and websocket reachability checks and
// creates state directories under CODEX_HOME, and it has to be pointed at a
// synthesised config to answer the question at all. Adding a release means
// repeating the observation and appending it here.
var verifiedAgentConcurrencyReleases = map[cliVersion]bool{
	{major: 0, minor: 153, patch: 4}: true,
}

// versionPattern matches the semantic triple inside `codex --version` output
// such as "codex-cli 0.153.4".
var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

type cliVersion struct {
	major int
	minor int
	patch int
}

// AgentConcurrencyNamespaceFor names the config table Autopus writes the
// ceiling into for a `codex --version` string, plus the evidence behind the
// choice. The table never varies with the version: AgentsNamespace is the only
// one the configuration reference documents, and no version comparison can
// establish that some other release wants a different table.
func AgentConcurrencyNamespaceFor(rawVersion string) (AgentConcurrencyNamespace, AgentConcurrencyEvidence) {
	parsed, ok := parseCLIVersion(rawVersion)
	if !ok {
		return AgentsNamespace, AgentConcurrencyUnknownVersion
	}
	if verifiedAgentConcurrencyReleases[parsed] {
		return AgentsNamespace, AgentConcurrencyVerifiedRelease
	}
	return AgentsNamespace, AgentConcurrencyAssumedDocumented
}

func parseCLIVersion(raw string) (cliVersion, bool) {
	match := versionPattern.FindStringSubmatch(raw)
	if match == nil {
		return cliVersion{}, false
	}
	parts := make([]int, 3)
	for i := range parts {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return cliVersion{}, false
		}
		parts[i] = value
	}
	return cliVersion{major: parts[0], minor: parts[1], patch: parts[2]}, true
}

// ProbeVersion reads `codex --version` under a bounded deadline. A missing
// binary or an unreadable response yields an empty version, which callers must
// treat as "unknown" instead of substituting a default release.
func ProbeVersion(ctx context.Context, binary string, timeout time.Duration) (string, bool) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--version")
	cmd.WaitDelay = processprobe.DefaultWaitDelay
	out, err := processprobe.Output(cmd)
	if err != nil {
		return "", false
	}
	if _, ok := parseCLIVersion(string(out)); !ok {
		return "", false
	}
	return string(out), true
}

// LoadedAgentConcurrencyUnknownReason names why the ceiling a running Codex
// session parsed cannot be read, which is what keeps a parsed config file from
// being reported as session state. Observed on codex-cli 0.153.4: `codex
// doctor --json` reports a config.load check carrying CODEX_HOME, the config
// path, parse status, model, and enabled feature flags, and no agents ceiling.
// `--strict-config` would answer the narrower question of whether a release
// recognises the key, but it is offered only by the interactive entry point
// and by `codex exec`, which runs a model turn.
const LoadedAgentConcurrencyUnknownReason = "no read-only Codex command reports the agent concurrency a " +
	"running session parsed ('codex doctor --json' reports config.load parse status without the agents " +
	"ceiling), so an on-disk value is configuration, not observed session state"

// EffectiveAgentConcurrencyUnknownReason names why the effective limit of a
// running session cannot be read. Beyond the loaded value being unreadable,
// the provider, the account, and the host all bound the request lower, and a
// successful config write is not evidence of added capacity.
const EffectiveAgentConcurrencyUnknownReason = "no read-only Codex command reports a running session's " +
	"agent concurrency (there is no 'codex config get', and 'codex doctor --json' omits the agents ceiling); " +
	"start a new Codex session for a config change to take effect"

// DescribeAgentConcurrencyNamespace renders the generated-config comment for a
// namespace decision: the count semantics, then the provenance of the table so
// no reader takes an assumption for a detected fact.
func DescribeAgentConcurrencyNamespace(
	namespace AgentConcurrencyNamespace,
	evidence AgentConcurrencyEvidence,
) string {
	semantics := "spawned agents only; the coordinator thread is not counted"
	switch evidence {
	case AgentConcurrencyVerifiedRelease:
		return fmt.Sprintf(
			"%s. The installed Codex release is verified to recognise this key in [%s]; the ceiling a running session applies is not readable",
			semantics, namespace,
		)
	case AgentConcurrencyUnknownVersion:
		return fmt.Sprintf(
			"%s. Codex CLI version unknown, assuming the documented [%s] namespace",
			semantics, namespace,
		)
	default:
		return fmt.Sprintf(
			"%s. The installed Codex release is not one Autopus verified, assuming the documented [%s] namespace",
			semantics, namespace,
		)
	}
}
