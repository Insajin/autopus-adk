package platform

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/config"
)

// --- TC1: AUTOPUS_PLATFORM=codex → RuntimeCodex, all Features false ----------------

func TestDetectRuntime_Codex(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "codex"})
	assert.Equal(t, RuntimeCodex, DetectRuntime())
}

func TestDetectFeatures_Codex_AllFalse(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "codex"})
	f := DetectFeatures()
	assert.False(t, f.Effort)
	assert.False(t, f.InitialPrompt)
	assert.False(t, f.Monitor)
	assert.False(t, f.TaskCreated)
	assert.False(t, f.HasOpus47)
}

// --- TC2: AUTOPUS_PLATFORM=claude-code + version 2.1.113 → features true ----------

func TestDetectFeatures_ClaudeCode_ValidVersion(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "claude-code"})
	patchExecCommand(t, fakeClaudeCommand("2.1.113 (Claude Code)"))
	withCC21Config(t, enabledCC21Features())

	f := DetectFeatures()
	assert.True(t, f.Effort)
	assert.True(t, f.InitialPrompt)
	assert.True(t, f.Monitor)
	assert.True(t, f.TaskCreated)
	assert.Equal(t, "2.1.113", f.ClaudeVersion)
}

func TestDetectFeatures_ClaudeCode_ConfigDisabled(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "claude-code"})
	patchExecCommand(t, fakeClaudeCommand("2.1.113"))
	withCC21Config(t, config.CC21FeaturesConf{})

	f := DetectFeatures()
	assert.False(t, f.Effort)
	assert.False(t, f.InitialPrompt)
	assert.False(t, f.Monitor)
	assert.False(t, f.TaskCreated)
	assert.Equal(t, "2.1.113", f.ClaudeVersion)
}

// --- TC3: AUTOPUS_PLATFORM=claude-code + version 2.0.50 → all false ---------------

func TestDetectFeatures_ClaudeCode_OldVersion(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "claude-code"})
	patchExecCommand(t, fakeClaudeCommand("2.0.50"))

	f := DetectFeatures()
	assert.False(t, f.Effort)
	assert.False(t, f.InitialPrompt)
	assert.False(t, f.Monitor)
	assert.False(t, f.TaskCreated)
	assert.Equal(t, "2.0.50", f.ClaudeVersion)
}

// --- TC4: no AUTOPUS_PLATFORM + no claude binary → RuntimeUnknown, all false ------

func TestDetectRuntime_Unknown_NoBinary(t *testing.T) {
	withoutEnv(t, "AUTOPUS_PLATFORM")
	// Override LookPath indirectly: patch execCommandFunc won't help here since
	// DetectRuntime calls exec.LookPath directly. We rely on the fact that
	// "claude" is not guaranteed in CI PATH; however the test may be flaky on
	// developer machines that have claude installed.
	//
	// The safest portable approach: this test validates the env-absent branch only
	// when claude is NOT in PATH.  Skip when it is present so the test stays green.
	if _, err := exec.LookPath("claude"); err == nil {
		t.Skip("claude binary present in PATH; TC4 requires its absence")
	}
	assert.Equal(t, RuntimeUnknown, DetectRuntime())
}

func TestDetectFeatures_Unknown_AllFalse(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "unknown-platform"})
	f := DetectFeatures()
	assert.False(t, f.Effort)
	assert.False(t, f.InitialPrompt)
	assert.False(t, f.Monitor)
	assert.False(t, f.TaskCreated)
}

// --- TC5~TC8: VersionSupportsCC21 ---------------------------------------------------

func TestVersionSupportsCC21_Above(t *testing.T) {
	assert.True(t, VersionSupportsCC21("2.1.113"))
}

func TestVersionSupportsCC21_Below(t *testing.T) {
	assert.False(t, VersionSupportsCC21("2.0.50"))
}

func TestVersionSupportsCC21_MajorThree(t *testing.T) {
	assert.True(t, VersionSupportsCC21("3.0.0"))
}

func TestVersionSupportsCC21_Invalid(t *testing.T) {
	assert.False(t, VersionSupportsCC21("invalid"))
}

func TestVersionSupportsCC21_ExactMinimum(t *testing.T) {
	assert.True(t, VersionSupportsCC21("2.1.0"))
}

func TestVersionSupportsCC21_WithSuffix(t *testing.T) {
	// Suffix text after a space must be stripped before parsing.
	assert.True(t, VersionSupportsCC21("2.1.113 (Claude Code)"))
}

// --- TC9: AUTOPUS_PLATFORM=claude-code + binary probe fails → Runtime=claude-code, features false

func TestDetectFeatures_ClaudeCode_BinaryProbeFails(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "claude-code"})
	patchExecCommand(t, failClaudeCommand())

	// claudeVersion will fail because exec returns non-zero.
	// DetectRuntime still returns claude-code (env takes priority).
	assert.Equal(t, RuntimeClaudeCode, DetectRuntime())

	f := DetectFeatures()
	assert.False(t, f.Effort)
	assert.False(t, f.InitialPrompt)
	assert.False(t, f.Monitor)
	assert.False(t, f.TaskCreated)
}

// --- TC10: CLAUDE_MODEL=claude-opus-4-7 → HasOpus47 true ---------------------------

func TestDetectFeatures_HasOpus47(t *testing.T) {
	withEnv(t, map[string]string{
		"AUTOPUS_PLATFORM": "claude-code",
		"CLAUDE_MODEL":     "claude-opus-4-7",
	})
	patchExecCommand(t, fakeClaudeCommand("2.1.113"))

	f := DetectFeatures()
	assert.True(t, f.HasOpus47)
}

func TestDetectFeatures_HasOpus47_ShortName(t *testing.T) {
	withEnv(t, map[string]string{
		"AUTOPUS_PLATFORM": "claude-code",
		"CLAUDE_MODEL":     "opus-4-7",
	})
	patchExecCommand(t, fakeClaudeCommand("2.1.113"))

	f := DetectFeatures()
	assert.True(t, f.HasOpus47)
}

func TestDetectFeatures_HasOpus47_False_WhenDifferentModel(t *testing.T) {
	withEnv(t, map[string]string{
		"AUTOPUS_PLATFORM": "claude-code",
		"CLAUDE_MODEL":     "claude-sonnet-4-6",
	})
	patchExecCommand(t, fakeClaudeCommand("2.1.113"))

	f := DetectFeatures()
	assert.False(t, f.HasOpus47)
}

// --- Runtime constant coverage ---------------------------------------------------

func TestRuntimeConstants(t *testing.T) {
	cases := []struct {
		env  string
		want Runtime
	}{
		{"claude-code", RuntimeClaudeCode},
		{"codex", RuntimeCodex},
		{"gemini-cli", RuntimeGeminiCLI},
		{"opencode", RuntimeOpenCode},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			withEnv(t, map[string]string{"AUTOPUS_PLATFORM": tc.env})
			assert.Equal(t, tc.want, DetectRuntime())
		})
	}
}

// --- MinClaudeVersion constant ---------------------------------------------------

func TestMinClaudeVersion_Constant(t *testing.T) {
	assert.True(t, VersionSupportsCC21(MinClaudeVersion))
}

// --- DetectRuntime heuristic (no env set) ----------------------------------------

func TestDetectRuntime_HeuristicClaudeBinary(t *testing.T) {
	// This path is only reachable when AUTOPUS_PLATFORM is unset and `claude` is
	// in PATH.  The test is skipped in environments without the binary.
	withoutEnv(t, "AUTOPUS_PLATFORM")
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not in PATH; skipping heuristic test")
	}
	assert.Equal(t, RuntimeClaudeCode, DetectRuntime())
}

// --- AUTOPUS_PLATFORM with unrecognised value → RuntimeUnknown ------------------

func TestDetectRuntime_UnrecognisedPlatform(t *testing.T) {
	withEnv(t, map[string]string{"AUTOPUS_PLATFORM": "some-unknown-cli"})
	assert.Equal(t, RuntimeUnknown, DetectRuntime())
}

// --- VersionSupportsCC21 edge cases ----------------------------------------------

func TestVersionSupportsCC21_OnlyMajor(t *testing.T) {
	// Only one part → false (not enough parts).
	assert.False(t, VersionSupportsCC21("2"))
}

func TestVersionSupportsCC21_MajorTwo_MinorZero(t *testing.T) {
	assert.False(t, VersionSupportsCC21("2.0.0"))
}

func TestVersionSupportsCC21_EmptyString(t *testing.T) {
	assert.False(t, VersionSupportsCC21(""))
}
