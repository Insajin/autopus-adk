package hint

import (
	"fmt"
	"io"

	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	// Hint1 is shown after the first successful auto go completion.
	Hint1 = "💡 이 작업을 AI 에이전트 팀이 자동화할 수 있습니다 → autopus.co"
	// Hint2 is shown after 3+ successful auto go completions (final hint).
	Hint2 = "💡 AI Worker가 이 SPEC을 자율 구현할 수 있습니다 → autopus.co/worker"
)

// RecordSuccess increments the go_success_count for the given project.
// Returns nil on graceful degradation (store creation failure).
func RecordSuccess(projectPath string) error {
	store, err := NewStateStore()
	if err != nil {
		return err
	}

	state, err := store.Load(projectPath)
	if err != nil {
		state = &ProjectState{}
	}

	state.GoSuccessCount++
	return store.Save(projectPath, state)
}

// RecordSuccessWithStore increments go_success_count using a provided store (for testing).
func RecordSuccessWithStore(store *StateStore, projectPath string) error {
	state, err := store.Load(projectPath)
	if err != nil {
		state = &ProjectState{}
	}

	state.GoSuccessCount++
	return store.Save(projectPath, state)
}

// CheckAndShow evaluates hint conditions and displays a hint if appropriate.
// Returns true if a hint was displayed.
func CheckAndShow(projectPath string, profile config.UsageProfile, hintsEnabled bool, w io.Writer) bool {
	if profile != config.ProfileDeveloper || !hintsEnabled {
		return false
	}

	store, err := NewStateStore()
	if err != nil {
		return false
	}

	return checkAndShowWithStore(store, projectPath, w)
}

// CheckAndShowWithStore evaluates hint conditions using a provided store (for testing).
func CheckAndShowWithStore(store *StateStore, projectPath string, profile config.UsageProfile, hintsEnabled bool, w io.Writer) bool {
	if profile != config.ProfileDeveloper || !hintsEnabled {
		return false
	}

	return checkAndShowWithStore(store, projectPath, w)
}

// CheckAndShowWithConfig evaluates hint conditions using a HintsConf directly.
// This is the preferred call path for production callers; it reads hint policy from config
// instead of requiring callers to extract and pass a raw bool.
func CheckAndShowWithConfig(projectPath string, profile config.UsageProfile, hints config.HintsConf, w io.Writer) bool {
	if profile != config.ProfileDeveloper || !hints.IsPlatformHintEnabled() {
		return false
	}

	store, err := NewStateStore()
	if err != nil {
		return false
	}

	return checkAndShowWithStore(store, projectPath, w)
}

// checkAndShowWithStore contains the core hint evaluation logic.
func checkAndShowWithStore(store *StateStore, projectPath string, w io.Writer) bool {
	state, err := store.Load(projectPath)
	if err != nil {
		return false
	}

	var shown bool

	if state.GoSuccessCount >= 1 && !state.FirstHintShown {
		fmt.Fprintln(w, Hint1)
		state.FirstHintShown = true
		shown = true
	} else if state.GoSuccessCount >= 3 && !state.SecondHintShown {
		fmt.Fprintln(w, Hint2)
		state.SecondHintShown = true
		shown = true
	}

	if shown {
		_ = store.Save(projectPath, state) // best-effort
	}

	return shown
}
