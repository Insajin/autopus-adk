package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
)

func mergeCfg(mode config.StatusLineMode) *config.HarnessConfig {
	cfg := &config.HarnessConfig{}
	cfg.Runtime.StatusLine.Mode = mode
	return cfg
}

func TestStatusLineState_Predicates(t *testing.T) {
	autopusReplace := StatusLineState{Command: autopusClaudeStatusLineCommand}
	autopusMerge := StatusLineState{Command: autopusClaudeCombinedStatusLineCommand}
	user := StatusLineState{Command: "my-custom-statusline.sh"}
	empty := StatusLineState{}

	if !autopusReplace.IsAutopusReplace() || autopusReplace.IsUserManaged() {
		t.Error("autopus replace command misclassified")
	}
	if !autopusMerge.IsAutopusMerge() || autopusMerge.IsUserManaged() {
		t.Error("autopus merge command misclassified")
	}
	if !user.IsUserManaged() || !user.HasCommand() {
		t.Error("user command must be user-managed")
	}
	if empty.HasCommand() || empty.IsUserManaged() {
		t.Error("empty command must not be user-managed")
	}
}

func TestResolveStatusLineMode_ExplicitConfigWins(t *testing.T) {
	got := resolveStatusLineMode(mergeCfg(config.StatusLineModeMerge), StatusLineState{Command: "x"})
	if got != config.StatusLineModeMerge {
		t.Errorf("explicit merge config = %q, want merge", got)
	}
}

func TestResolveStatusLineMode_InferFromExisting(t *testing.T) {
	cases := []struct {
		existing StatusLineState
		want     config.StatusLineMode
	}{
		{StatusLineState{Command: autopusClaudeCombinedStatusLineCommand}, config.StatusLineModeMerge},
		{StatusLineState{Command: autopusClaudeStatusLineCommand}, config.StatusLineModeReplace},
		{StatusLineState{}, config.StatusLineModeReplace},
		{StatusLineState{Command: "user.sh"}, config.StatusLineModeKeep},
	}
	for _, tc := range cases {
		got := resolveStatusLineMode(&config.HarnessConfig{}, tc.existing)
		if got != tc.want {
			t.Errorf("infer mode for %q = %q, want %q", tc.existing.Command, got, tc.want)
		}
	}
}

func TestResolveMergedUserStatusLineCommand(t *testing.T) {
	dir := t.TempDir()
	a := NewWithRoot(dir)

	// Non-merge mode returns empty.
	if got := a.resolveMergedUserStatusLineCommand(StatusLineState{Command: "u.sh"}, config.StatusLineModeReplace); got != "" {
		t.Errorf("non-merge mode returned %q, want empty", got)
	}
	// User-managed command is preserved directly.
	if got := a.resolveMergedUserStatusLineCommand(StatusLineState{Command: "u.sh"}, config.StatusLineModeMerge); got != "u.sh" {
		t.Errorf("user command merge = %q, want u.sh", got)
	}
	// Non-merge existing, not user-managed (autopus replace) returns empty.
	if got := a.resolveMergedUserStatusLineCommand(StatusLineState{Command: autopusClaudeStatusLineCommand}, config.StatusLineModeMerge); got != "" {
		t.Errorf("autopus-replace merge = %q, want empty", got)
	}

	// Already-merge existing reads preserved user command file.
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "statusline-user-command.txt"), []byte("preserved.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := a.resolveMergedUserStatusLineCommand(StatusLineState{Command: autopusClaudeCombinedStatusLineCommand}, config.StatusLineModeMerge)
	if got != "preserved.sh" {
		t.Errorf("merge from file = %q, want preserved.sh", got)
	}
}

func TestDefaultClaudeStatusLineBuilders(t *testing.T) {
	combined := defaultClaudeCombinedStatusLine()
	if combined["command"] != autopusClaudeCombinedStatusLineCommand {
		t.Errorf("combined command = %v", combined["command"])
	}
	if combined["type"] != "command" || combined["padding"] != 1 {
		t.Errorf("combined shape = %+v", combined)
	}
}

func TestInspectStatusLine(t *testing.T) {
	dir := t.TempDir()
	// Missing settings file returns zero state.
	if got := InspectStatusLine(dir); got.HasCommand() {
		t.Error("missing settings must yield empty state")
	}

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Settings with a statusLine command.
	settings := `{"statusLine":{"command":"  my.sh  "}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	st := InspectStatusLine(dir)
	if st.Command != "my.sh" {
		t.Errorf("inspected command = %q, want trimmed my.sh", st.Command)
	}

	// Invalid JSON yields empty state.
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := InspectStatusLine(dir); got.HasCommand() {
		t.Error("invalid JSON must yield empty state")
	}
}

func TestStatusLineStateFromValue(t *testing.T) {
	v := map[string]any{"command": "  x.sh "}
	if got := statusLineStateFromValue(v); got.Command != "x.sh" {
		t.Errorf("fromValue = %q", got.Command)
	}
	if got := statusLineStateFromValue("not-a-map"); got.HasCommand() {
		t.Error("non-map value must yield empty state")
	}
}
