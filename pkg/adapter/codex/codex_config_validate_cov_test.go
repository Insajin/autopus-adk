package codex

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
)

// hasValidationMessage reports whether errs contains a message substring.
func hasValidationMessage(errs []adapter.ValidationError, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidateDeprecatedConfigKeys_ApprovalMode(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateDeprecatedConfigKeys("approval_mode = \"auto\"\n", &errs)
	assert.True(t, hasValidationMessage(errs, "deprecated approval_mode"))
}

func TestValidateDeprecatedConfigKeys_SandboxTable(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateDeprecatedConfigKeys("[sandbox]\nmode = \"x\"\n", &errs)
	assert.True(t, hasValidationMessage(errs, "deprecated [sandbox] table"))
}

func TestValidateDeprecatedConfigKeys_Clean(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateDeprecatedConfigKeys("approval_policy = \"on-failure\"\nsandbox_mode = \"x\"\n", &errs)
	assert.Empty(t, errs)
}

func TestValidateProjectDocBudget_Missing(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateProjectDocBudget("model = \"gpt\"\n", &errs)
	assert.True(t, hasValidationMessage(errs, "project_doc_max_bytes 설정이 없음"))
}

func TestValidateProjectDocBudget_TooLow(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateProjectDocBudget("project_doc_max_bytes = 100\n", &errs)
	assert.True(t, hasValidationMessage(errs, "너무 낮음"))
}

func TestValidateProjectDocBudget_Adequate(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateProjectDocBudget("project_doc_max_bytes = 1048576\n", &errs)
	assert.Empty(t, errs)
}

func TestParseProjectDocMaxBytes_Valid(t *testing.T) {
	t.Parallel()
	v, ok := parseProjectDocMaxBytes("project_doc_max_bytes = 4096\n")
	assert.True(t, ok)
	assert.Equal(t, 4096, v)
}

func TestParseProjectDocMaxBytes_NonNumeric(t *testing.T) {
	t.Parallel()
	v, ok := parseProjectDocMaxBytes("project_doc_max_bytes = abc\n")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestParseProjectDocMaxBytes_NoAssignment(t *testing.T) {
	t.Parallel()
	// Line starts with key but has no '=' delimiter.
	v, ok := parseProjectDocMaxBytes("project_doc_max_bytes\n")
	assert.False(t, ok)
	assert.Equal(t, 0, v)
}

func TestParseProjectDocMaxBytes_Absent(t *testing.T) {
	t.Parallel()
	_, ok := parseProjectDocMaxBytes("model = \"gpt\"\n")
	assert.False(t, ok)
}

func TestContainsConfigKey_SkipsCommentsAndBlank(t *testing.T) {
	t.Parallel()
	content := "\n# approval_mode = old\nmodel = \"gpt\"\n"
	assert.False(t, containsConfigKey(content, "approval_mode"))
}

func TestContainsConfigKey_MatchesSpaceAndEquals(t *testing.T) {
	t.Parallel()
	assert.True(t, containsConfigKey("approval_mode = x\n", "approval_mode"))
	assert.True(t, containsConfigKey("approval_mode=x\n", "approval_mode"))
}

func TestValidateBundledCodexPlugins_Enabled(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	content := "[plugins.\"browser-use@openai-bundled\"]\nenabled = true\n"
	validateBundledCodexPlugins(content, &errs)
	assert.Empty(t, errs)
}

func TestValidateBundledCodexPlugins_Disabled(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	validateBundledCodexPlugins("model = \"gpt\"\n", &errs)
	assert.True(t, hasValidationMessage(errs, "browser-use plugin이 enabled 상태가 아님"))
}

func TestValidateCodexFeatureFlags_AllEnabled(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	content := "[features]\ngoals = true\nmulti_agent = true\n"
	validateCodexFeatureFlags(content, &errs)
	assert.Empty(t, errs)
}

func TestValidateCodexFeatureFlags_MissingGoals(t *testing.T) {
	t.Parallel()
	var errs []adapter.ValidationError
	content := "[features]\nmulti_agent = true\n"
	validateCodexFeatureFlags(content, &errs)
	assert.True(t, hasValidationMessage(errs, "goals feature가 enabled 상태가 아님"))
}

func TestSectionHasKeyValue_WrongSection(t *testing.T) {
	t.Parallel()
	content := "[other]\nenabled = true\n"
	assert.False(t, sectionHasEnabledTrue(content, "features"))
}

func TestSectionHasKeyValue_Found(t *testing.T) {
	t.Parallel()
	content := "[features]\nenabled = true\n"
	assert.True(t, sectionHasEnabledTrue(content, "features"))
}
