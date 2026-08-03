package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
)

// @AX:NOTE [AUTO]: 128 KiB output and eight recursive levels bound untrusted OMP config-list evidence.
const ompContextDoctorProbeOutput = 128 * 1024

type ompContextDoctorRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

func probeOMPContextCurrentRuntime(ctx context.Context, runner ompContextDoctorRunner) ompContextCurrentProbe {
	if runner == nil {
		return ompContextCurrentProbe{Reason: "identity_unverified"}
	}
	version, err := runOMPContextDoctorCommand(ctx, runner, "--version")
	identity := strings.TrimSpace(string(version))
	probe := ompContextCurrentProbe{Version: identity, IdentityVerified: err == nil && ompDoctorVersion.MatchString(identity)}
	if !probe.IdentityVerified {
		probe.Version, probe.Reason = "", "identity_unverified"
		return probe
	}
	configList, err := runOMPContextDoctorCommand(ctx, runner, "config", "list", "--json")
	if err != nil {
		probe.Reason = "config_readback_unproved"
		return probe
	}
	probe.ConfigListSchema, probe.CompactionSchema, probe.MemorySchema = inspectOMPContextConfigList(configList)
	probe.OverlayReadback = probe.ConfigListSchema && probe.CompactionSchema && probe.MemorySchema
	if !probe.OverlayReadback {
		probe.Reason = "config_schema_unproved"
	}
	return probe
}

func runOMPContextDoctorCommand(ctx context.Context, runner ompContextDoctorRunner, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, ompDoctorProbeTimeout)
	defer cancel()
	output, err := runner.Run(probeCtx, "omp", args...)
	if err != nil {
		return nil, err
	}
	if len(output) > ompContextDoctorProbeOutput {
		return nil, errors.New("OMP context doctor output exceeded limit")
	}
	return output, nil
}

func inspectOMPContextConfigList(data []byte) (valid, compaction, memory bool) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || requireOMPContextDoctorJSONEOF(decoder) != nil {
		return false, false, false
	}
	root, ok := value.(map[string]any)
	if !ok {
		return false, false, false
	}
	compaction = findOMPContextConfigSetting(root, "compaction", 0)
	memory = findOMPContextConfigSetting(root, "memory", 0)
	return compaction && memory, compaction, memory
}

// @AX:WARN [AUTO]: OMP config schema discovery has cyclomatic complexity 16.
// @AX:REASON [AUTO]: exact nested and flat keys, bounded recursion, heterogeneous JSON nodes, and installed-version compatibility converge here.
func findOMPContextConfigSetting(value any, target string, depth int) bool {
	if depth > 8 {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		if setting, ok := typed[target]; ok && validOMPContextConfigSetting(setting) {
			return true
		}
		flatKey := map[string]string{"compaction": "compaction.enabled", "memory": "memory.backend"}[target]
		if setting, ok := typed[flatKey]; flatKey != "" && ok && validOMPContextConfigSetting(setting) {
			return true
		}
		if key, ok := typed["key"].(string); ok && key == target && validOMPContextConfigSetting(typed["value"]) {
			return true
		}
		for _, child := range typed {
			if findOMPContextConfigSetting(child, target, depth+1) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if findOMPContextConfigSetting(child, target, depth+1) {
				return true
			}
		}
	}
	return false
}

func validOMPContextConfigSetting(value any) bool {
	switch value.(type) {
	case map[string]any, bool:
		return true
	default:
		return false
	}
}

func buildOMPContextDoctorInput(
	ctx context.Context,
	root string,
	cfg *config.HarnessConfig,
	runner ompContextDoctorRunner,
) ompContextDoctorInput {
	policy, selected, err := workflowContextPolicyFromConfig(cfg)
	if err != nil || !selected {
		return ompContextDoctorInput{}
	}
	now := time.Now().UTC()
	return ompContextDoctorInput{
		Enabled: true, Profile: policy.Profile, RequestedHistoryMode: policy.HistoryMode,
		RequestedMemoryMode: policy.MemoryMode, FallbackMode: policy.Fallback,
		RuntimeRootPolicy: policy.RuntimeRootPolicy, Current: probeOMPContextCurrentRuntime(ctx, runner),
		Receipt: readOMPContextDoctorReceipt(root, now),
	}
}

func probeOMPContextDoctor(ctx context.Context, root string) ompContextDoctorReport {
	cfg, err := config.LoadPreview(root)
	if err != nil {
		return ompContextDoctorReport{}
	}
	input := buildOMPContextDoctorInput(ctx, root, cfg, ompModelDoctorExecRunner{root: root})
	return checkOMPContextDoctor(input)
}
