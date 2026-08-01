package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// shippedRuleClassification is the S11 expected classification per rule.
var shippedRuleClassification = map[string]string{
	"branding":            "always",
	"context7-docs":       "always",
	"deferred-tools":      "always",
	"doc-storage":         "always",
	"language-policy":     "always",
	"objective-reasoning": "always",
	"project-identity":    "always",
	"spec-quality":        "always",
	"subagent-delegation": "always",
	"techstack-freshness": "always",
	"file-size-limit":     "paths-scoped",
	"lore-commit":         "hook-fired",
	"shell-portability":   "hook-fired",
	"worktree-safety":     "hook-fired",
}

// runRulesList executes `auto rules list` and returns its stdout.
func runRulesList(t *testing.T) string {
	t.Helper()
	cmd := newRulesCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	return out.String()
}

// rowFor returns the printed row whose first column is the rule name.
func rowFor(t *testing.T, output, name string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return line
		}
	}
	t.Fatalf("no row printed for rule %q in:\n%s", name, output)
	return ""
}

// TestRulesList_ClassificationIsInspectable is the S11 oracle for
// REQ-CONDRULE-OBS-01.
func TestRulesList_ClassificationIsInspectable(t *testing.T) {
	output := runRulesList(t)

	counts := map[string]int{}
	for name, want := range shippedRuleClassification {
		row := rowFor(t, output, name)
		assert.Contains(t, row, want, "rule %s classification", name)
		counts[want]++
	}
	assert.Equal(t, 3, counts["hook-fired"])
	assert.Equal(t, 1, counts["paths-scoped"])
	assert.Equal(t, 10, counts["always"])

	lore := rowFor(t, output, "lore-commit")
	assert.Contains(t, lore, "tool:bash", "hook-fired rules print their trigger")
	assert.Contains(t, lore, ".claude/hooks/autopus/conditional/lore-commit.md",
		"hook-fired rules print a conditional body destination")

	fileSize := rowFor(t, output, "file-size-limit")
	assert.Contains(t, fileSize, ".claude/rules/autopus/file-size-limit.md")

	branding := rowFor(t, output, "branding")
	assert.Contains(t, branding, ".claude/rules/autopus/branding.md")
}

// TestRulesFire_MissingManifestExitsZeroWithNoOutput keeps the CLI entry point
// aligned with the REQ-CONDRULE-FIRE-03 fail-open contract.
func TestRulesFire_MissingManifestExitsZeroWithNoOutput(t *testing.T) {
	t.Chdir(t.TempDir())

	cmd := newRulesCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(
		`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m x"}}`))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"fire", "--event", "PreToolUse"})

	require.NoError(t, cmd.Execute(), "the dispatcher must never block a tool call")
	assert.Empty(t, out.String())
	assert.Empty(t, errOut.String())
}

// runRulesFire executes `auto rules fire` in dir with the given stdin and
// returns stdout and stderr.
func runRulesFire(t *testing.T, dir, stdin string) (string, string) {
	t.Helper()
	t.Chdir(dir)

	cmd := newRulesCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"fire", "--event", "PreToolUse"})

	require.NoError(t, cmd.Execute(), "the dispatcher must never block a tool call")
	return out.String(), errOut.String()
}

// TestRulesFire_InjectsMatchedRuleThroughTheCLI is the S1 oracle at the command
// boundary the generated PreToolUse hook actually invokes. The pkg/rulecond
// tests call Fire directly, so without this the CLI could resolve the wrong
// root, drop stdin, or write to the wrong stream and still ship green.
func TestRulesFire_InjectsMatchedRuleThroughTheCLI(t *testing.T) {
	dir := t.TempDir()
	bodyRoot := filepath.Join(dir, filepath.FromSlash(rulecond.BodyRootRelPath))
	require.NoError(t, os.MkdirAll(bodyRoot, 0o755))

	const marker = "CLI-LORE-BODY"
	require.NoError(t, os.WriteFile(filepath.Join(bodyRoot, "lore-commit.md"),
		[]byte("# Lore Commit\n\n"+marker+"\n"), 0o644))

	manifest := rulecond.Manifest{Version: "1", Rules: []rulecond.ManifestRule{{
		Name:       "lore-commit",
		Event:      "PreToolUse",
		Matcher:    "Bash",
		Conditions: []string{`\bgit\s+commit\b`},
		Body:       "lore-commit.md",
	}}}
	blob, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, filepath.FromSlash(rulecond.ManifestRelPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, blob, 0o644))

	stdout, stderr := runRulesFire(t, dir,
		`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m \"feat(x): y\""}}`)

	var payload struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload),
		"the CLI must print a JSON hook payload on stdout")

	assert.Equal(t, "PreToolUse", payload.HookSpecificOutput.HookEventName)
	assert.Contains(t, payload.HookSpecificOutput.AdditionalContext, marker,
		"the matched rule body must reach additionalContext")
	assert.Contains(t, payload.HookSpecificOutput.AdditionalContext, "lore-commit",
		"the matched rule must be named")
	assert.NotContains(t, payload.HookSpecificOutput.AdditionalContext, "git commit -m",
		"tool input must never re-enter context")
	assert.Empty(t, stderr)
}

// TestRulesFire_NonMatchingCommandStaysSilentThroughTheCLI is the negative
// control: the CLI must not emit a payload when nothing matches.
func TestRulesFire_NonMatchingCommandStaysSilentThroughTheCLI(t *testing.T) {
	dir := t.TempDir()
	bodyRoot := filepath.Join(dir, filepath.FromSlash(rulecond.BodyRootRelPath))
	require.NoError(t, os.MkdirAll(bodyRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bodyRoot, "lore-commit.md"),
		[]byte("# Lore Commit\n\nBODY\n"), 0o644))

	blob, err := json.Marshal(rulecond.Manifest{Version: "1", Rules: []rulecond.ManifestRule{{
		Name:       "lore-commit",
		Event:      "PreToolUse",
		Matcher:    "Bash",
		Conditions: []string{`\bgit\s+commit\b`},
		Body:       "lore-commit.md",
	}}})
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, filepath.FromSlash(rulecond.ManifestRelPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0o755))
	require.NoError(t, os.WriteFile(manifestPath, blob, 0o644))

	stdout, stderr := runRulesFire(t, dir,
		`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"}}`)

	assert.Empty(t, stdout, "a non-match must produce no payload")
	assert.Empty(t, stderr)
}

// TestRulesCmd_RegisteredOnRoot covers the T6 wiring.
func TestRulesCmd_RegisteredOnRoot(t *testing.T) {
	var found []string
	for _, cmd := range NewRootCmd().Commands() {
		if cmd.Name() == "rules" {
			for _, sub := range cmd.Commands() {
				found = append(found, sub.Name())
			}
		}
	}
	assert.Subset(t, found, []string{"fire", "list"},
		"auto rules must expose fire and list")
}

// TestHygieneSourceMapping_ConditionalPaths covers T9: the generated conditional
// paths map back to the rule source, so drift detection does not orphan them.
func TestHygieneSourceMapping_ConditionalPaths(t *testing.T) {
	for _, generated := range []string{
		".claude/hooks/autopus/conditional/lore-commit.md",
		".claude/hooks/autopus/conditional-rules.json",
	} {
		prefixes := sourcePrefixesForGenerated(generated)
		assert.Contains(t, prefixes, "content/rules/", "source mapping for %s", generated)
		assert.True(t, sourceMatchesGenerated(generated, "content/rules/lore-commit.md"),
			"%s must be attributable to a rule source edit", generated)
	}
}

// TestResolveProjectRoot_RequiresMarkerAndStopsAtHome pins REQ-CONDRULE-FIRE-11
// root resolution. The bare working directory is an implicit redirection
// channel, so a marker is required, and the home directory is never itself a
// project root even though `~/.claude` exists on most installs.
func TestResolveProjectRoot_RequiresMarkerAndStopsAtHome(t *testing.T) {
	tests := []struct {
		name        string
		markerDir   string // relative to home; "-" creates no marker at all
		markerName  string
		markerIsLnk bool   // ship the marker as a symlink, as a clone can
		startDir    string // relative to home
		wantDir     string // relative to home; empty means unresolved
	}{
		{name: "claude marker in the start directory", markerDir: "proj", markerName: ".claude", startDir: "proj", wantDir: "proj"},
		{name: "git marker in an ancestor", markerDir: "proj", markerName: ".git", startDir: "proj/pkg/sub", wantDir: "proj"},
		{name: "no marker anywhere", markerDir: "-", startDir: "work/sub"},
		{name: "home marker is not a project root", markerDir: ".", markerName: ".claude", startDir: "work/sub"},
		// A symlinked marker is not evidence of a project root: accepting it
		// would anchor the conditional surface outside the checkout.
		{name: "symlinked claude marker is refused", markerDir: "proj", markerName: ".claude", markerIsLnk: true, startDir: "proj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			start := filepath.Join(home, filepath.FromSlash(tt.startDir))
			require.NoError(t, os.MkdirAll(start, 0o755))
			if tt.markerDir != "-" {
				marker := filepath.Join(home, filepath.FromSlash(tt.markerDir), tt.markerName)
				require.NoError(t, os.MkdirAll(filepath.Dir(marker), 0o755))
				if tt.markerIsLnk {
					outside := t.TempDir()
					if err := os.Symlink(outside, marker); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				} else {
					require.NoError(t, os.MkdirAll(marker, 0o755))
				}
			}

			got, ok := resolveProjectRoot(start)

			if tt.wantDir == "" {
				assert.False(t, ok, "resolution must fail open outside a project")
				assert.Empty(t, got)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, filepath.Join(home, filepath.FromSlash(tt.wantDir)), got)
		})
	}
}
