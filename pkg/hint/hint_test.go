package hint

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
)

func newTestStore(t *testing.T) *StateStore {
	t.Helper()
	dir := t.TempDir()
	return NewStateStoreWithPath(filepath.Join(dir, "state.json"))
}

func TestRecordSuccessWithStore(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatalf("RecordSuccessWithStore() error: %v", err)
	}

	state, err := store.Load(project)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if state.GoSuccessCount != 1 {
		t.Errorf("GoSuccessCount = %d, want 1", state.GoSuccessCount)
	}

	// Record again
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatalf("RecordSuccessWithStore() second call error: %v", err)
	}
	state, _ = store.Load(project)
	if state.GoSuccessCount != 2 {
		t.Errorf("GoSuccessCount = %d, want 2", state.GoSuccessCount)
	}
}

func TestCheckAndShow_FullstackProfile(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// Record a success so hint would trigger for developer
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileFullstack, true, &buf)
	if shown {
		t.Error("hint shown for fullstack profile, want no hint")
	}
	if buf.Len() != 0 {
		t.Errorf("output = %q, want empty", buf.String())
	}
}

func TestCheckAndShow_HintsDisabled(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, false, &buf)
	if shown {
		t.Error("hint shown with hints disabled, want no hint")
	}
}

func TestCheckAndShow_FirstHint(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// Record 1 success
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if !shown {
		t.Fatal("hint not shown after first success, want first hint")
	}
	if !strings.Contains(buf.String(), Hint1) {
		t.Errorf("output = %q, want to contain %q", buf.String(), Hint1)
	}

	// Verify state was updated
	state, _ := store.Load(project)
	if !state.FirstHintShown {
		t.Error("FirstHintShown = false after showing first hint")
	}
}

func TestCheckAndShow_FirstHintNotRepeated(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)

	// Call again — first hint should not repeat
	buf.Reset()
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Error("first hint repeated on second call, want no hint")
	}
}

func TestCheckAndShow_SecondHintAt3(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// Record 1 success and show first hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	var discard bytes.Buffer
	CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &discard)

	// Record 2 more successes (total 3)
	for i := 0; i < 2; i++ {
		if err := RecordSuccessWithStore(store, project); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if !shown {
		t.Fatal("second hint not shown at count=3")
	}
	if !strings.Contains(buf.String(), Hint2) {
		t.Errorf("output = %q, want to contain %q", buf.String(), Hint2)
	}

	// Verify state
	state, _ := store.Load(project)
	if !state.SecondHintShown {
		t.Error("SecondHintShown = false after showing second hint")
	}
}

func TestCheckAndShow_NoHintAfterBoth(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// Show first hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	var discard bytes.Buffer
	CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &discard)

	// Show second hint
	for i := 0; i < 2; i++ {
		if err := RecordSuccessWithStore(store, project); err != nil {
			t.Fatal(err)
		}
	}
	discard.Reset()
	CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &discard)

	// Record more and check — no more hints
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Error("hint shown after both hints exhausted, want no hint")
	}
}

func TestCheckAndShow_ZeroSuccessNoHint(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// No RecordSuccess — count is 0
	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Error("hint shown with zero successes, want no hint")
	}
}

func TestCheckAndShow_EmptyProfileDefaultNone(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	// Empty profile is NOT ProfileDeveloper, so no hint
	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, "", true, &buf)
	if shown {
		t.Error("hint shown for empty profile, want no hint (caller must use Effective())")
	}
}

func TestCheckAndShow_SecondHintBoundary(t *testing.T) {
	store := newTestStore(t)
	project := "/test/project"

	// Record 1 success, show first hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	var discard bytes.Buffer
	CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &discard)

	// Record 1 more (total 2) — not enough for second hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Error("second hint shown at count=2, want no hint until count>=3")
	}
}
