package config

import "fmt"

// Codex counts spawned-agent threads without the primary thread, so these
// bounds describe workers only: the coordinator session is always extra.
// https://learn.chatgpt.com/docs/config-file/config-reference documents
// agents.max_concurrent_threads_per_session as "excluding the primary thread".
const (
	// CodexAgentConcurrencyDefault is the spawned-worker ceiling Autopus
	// requests when the project states nothing. Four workers plus the
	// coordinator is the historical harness default.
	CodexAgentConcurrencyDefault = 4
	// CodexAgentConcurrencyMin keeps at least one spawnable worker: zero
	// workers would disable team mode through a value that reads like a
	// tuning knob rather than a feature switch.
	CodexAgentConcurrencyMin = 1
	// CodexAgentConcurrencyMax bounds the request far above any measured
	// useful fan-out so a typo cannot ask a host for unbounded threads.
	CodexAgentConcurrencyMax = 64
)

// CodexConf holds the project-scoped Codex CLI settings Autopus owns.
type CodexConf struct {
	Agents CodexAgentsConf `yaml:"agents,omitempty"`
}

// CodexAgentsConf configures the Codex multi-agent surface.
type CodexAgentsConf struct {
	// MaxConcurrentThreads is the number of spawned workers Autopus requests,
	// excluding the coordinator thread. Zero means "use the harness default",
	// which is what an autopus.yaml written before this setting existed says.
	MaxConcurrentThreads int `yaml:"max_concurrent_threads,omitempty"`
}

// CodexAgentConcurrency resolves the requested spawned-worker ceiling. It is a
// request, not an observed capacity: the host and the account can both bound
// the running session lower.
func (c *HarnessConfig) CodexAgentConcurrency() int {
	if c == nil || c.Codex.Agents.MaxConcurrentThreads == 0 {
		return CodexAgentConcurrencyDefault
	}
	return c.Codex.Agents.MaxConcurrentThreads
}

// Validate rejects an out-of-range explicit worker ceiling.
func (c CodexConf) Validate() error {
	requested := c.Agents.MaxConcurrentThreads
	if requested == 0 {
		return nil
	}
	if requested < CodexAgentConcurrencyMin || requested > CodexAgentConcurrencyMax {
		return fmt.Errorf(
			"codex.agents.max_concurrent_threads %d is out of range: must be between %d and %d",
			requested, CodexAgentConcurrencyMin, CodexAgentConcurrencyMax,
		)
	}
	return nil
}
