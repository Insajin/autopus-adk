package worker

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/insajin/autopus-adk/pkg/telemetry"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

var usageIdentitySequence atomic.Uint64

type usageProvenance struct {
	ProviderVersion string
	ModelVersion    string
	RiskPolicy      string
	CacheStratum    string
	ConfigHash      string
}

func usageProvenanceFromTask(task adapter.TaskConfig) usageProvenance {
	return usageProvenance{
		ProviderVersion: task.ProviderVersion,
		ModelVersion:    task.ModelVersion,
		RiskPolicy:      task.RiskPolicy,
		CacheStratum:    task.CacheStratum,
		ConfigHash:      task.ConfigHash,
	}
}

func (p usageProvenance) applyTask(task *adapter.TaskConfig) {
	task.ProviderVersion = p.ProviderVersion
	task.ModelVersion = p.ModelVersion
	task.RiskPolicy = p.RiskPolicy
	task.CacheStratum = p.CacheStratum
	task.ConfigHash = p.ConfigHash
}

func (p usageProvenance) applyEnvelope(input *telemetry.UsageEnvelope) {
	input.ProviderVersion = p.ProviderVersion
	input.ModelVersion = p.ModelVersion
	input.RiskPolicy = p.RiskPolicy
	input.CacheStratum = p.CacheStratum
	input.ConfigHash = p.ConfigHash
}

func ensureUsageIdentity(task adapter.TaskConfig, phase, role string) adapter.TaskConfig {
	if task.Attempt <= 0 {
		task.Attempt = 1
	}
	if task.Phase == "" {
		task.Phase = phase
	}
	if task.Role == "" {
		task.Role = role
	}
	if task.RunID == "" {
		sequence := usageIdentitySequence.Add(1)
		task.RunID = fmt.Sprintf("%s-%d-%d", task.TaskID, time.Now().UnixNano(), sequence)
	}
	if task.CallID == "" {
		sequence := usageIdentitySequence.Add(1)
		task.CallID = fmt.Sprintf("%s:%s:%d:%d", task.RunID, task.Phase, task.Attempt, sequence)
	}
	return task
}

func bindUsageIdentity(inputs []telemetry.UsageEnvelope, task adapter.TaskConfig, providerName ...string) []telemetry.UsageEnvelope {
	if len(inputs) == 0 {
		return nil
	}
	bound := make([]telemetry.UsageEnvelope, len(inputs))
	provenance := usageProvenanceFromTask(task)
	for i, input := range inputs {
		input.RunID = task.RunID
		input.CallID = task.CallID
		input.TaskID = task.TaskID
		input.Attempt = task.Attempt
		if len(providerName) > 0 {
			input.Provider = providerName[0]
		}
		input.Model = task.Model
		input.Effort = task.Effort
		input.Phase = task.Phase
		input.Role = task.Role
		provenance.applyEnvelope(&input)
		bound[i] = input
	}
	return bound
}

func mergeBoundUsage(existing, incoming []telemetry.UsageEnvelope) []telemetry.UsageEnvelope {
	merged := append([]telemetry.UsageEnvelope(nil), existing...)
	for _, candidate := range incoming {
		matched := false
		for _, prior := range merged {
			if prior.RunID != candidate.RunID || prior.CallID != candidate.CallID {
				continue
			}
			matched = !telemetry.AggregateUsage([]telemetry.UsageEnvelope{prior, candidate}).PromotionBlocked
			break
		}
		if !matched {
			merged = append(merged, candidate)
		}
	}
	return merged
}
