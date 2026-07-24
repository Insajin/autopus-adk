package workflow

import "testing"

func TestIsSafeAgentModel_Opus5AndLegacyOpus48AreAllowed(t *testing.T) {
	t.Parallel()

	for _, model := range []string{"claude-opus-5", "claude-opus-4-8"} {
		if !isSafeAgentModel(model) {
			t.Errorf("isSafeAgentModel(%q) = false, want true", model)
		}
	}
	if isSafeAgentModel(`claude-opus-5");evil((`) {
		t.Error("Opus 5 injection payload must be rejected")
	}
}
