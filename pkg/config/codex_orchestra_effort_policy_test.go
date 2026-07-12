package config

import "testing"

func TestCodexOrchestraEffortPolicy(t *testing.T) {
	quality := QualityConf{Default: "ultra"}

	if got, want := quality.CodexOrchestraProfile(), (CodexProfile{Model: CodexSolModel, Effort: CodexEffortMax}); got != want {
		t.Errorf("CodexOrchestraProfile() = %#v, want %#v", got, want)
	}

	if got, want := quality.CodexSupervisorProfile(), (CodexProfile{Model: CodexSolModel, Effort: CodexEffortUltra}); got != want {
		t.Errorf("CodexSupervisorProfile() = %#v, want %#v", got, want)
	}

	if got, want := CodexOrchestraTimeoutSeconds, 420; got != want {
		t.Errorf("CodexOrchestraTimeoutSeconds = %d, want %d", got, want)
	}
}
