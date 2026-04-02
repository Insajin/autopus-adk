package hint

import (
	"bytes"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestRecordSuccess_RealStore(t *testing.T) {
	// Tests RecordSuccess and CheckAndShow using real NewStateStore (hits ~/.autopus/).
	project := t.TempDir() // unique project path

	if err := RecordSuccess(project); err != nil {
		t.Fatalf("RecordSuccess() error: %v", err)
	}

	var buf bytes.Buffer
	shown := CheckAndShow(project, config.ProfileDeveloper, true, &buf)
	if !shown {
		t.Error("CheckAndShow() did not show hint after RecordSuccess")
	}
	if !strings.Contains(buf.String(), Hint1) {
		t.Errorf("output = %q, want Hint1", buf.String())
	}
}

func TestCheckAndShow_RealStore_FullstackGuard(t *testing.T) {
	project := t.TempDir()

	if err := RecordSuccess(project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShow(project, config.ProfileFullstack, true, &buf)
	if shown {
		t.Error("CheckAndShow() showed hint for fullstack profile")
	}
}

func TestCheckAndShow_RealStore_HintsDisabledGuard(t *testing.T) {
	project := t.TempDir()

	if err := RecordSuccess(project); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	shown := CheckAndShow(project, config.ProfileDeveloper, false, &buf)
	if shown {
		t.Error("CheckAndShow() showed hint with hints disabled")
	}
}

func TestRecordSuccess_ConsecutiveCalls(t *testing.T) {
	store := newTestStore(t)
	project := "/test/consecutive"

	for i := 1; i <= 5; i++ {
		if err := RecordSuccessWithStore(store, project); err != nil {
			t.Fatalf("call %d: RecordSuccessWithStore() error: %v", i, err)
		}
		state, _ := store.Load(project)
		if state.GoSuccessCount != i {
			t.Errorf("call %d: GoSuccessCount = %d, want %d", i, state.GoSuccessCount, i)
		}
	}
}

func TestIntegration_FullFlow(t *testing.T) {
	store := newTestStore(t)
	project := "/test/integration"

	// Step 1: first go success -> first hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	shown := CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if !shown || !strings.Contains(buf.String(), Hint1) {
		t.Fatalf("step 1: want Hint1, got shown=%v output=%q", shown, buf.String())
	}

	// Step 2: second go success -> no hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	shown = CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Fatal("step 2: unexpected hint at count=2")
	}

	// Step 3: third go success -> second hint
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	shown = CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if !shown || !strings.Contains(buf.String(), Hint2) {
		t.Fatalf("step 3: want Hint2, got shown=%v output=%q", shown, buf.String())
	}

	// Step 4: fourth go success -> no more hints
	if err := RecordSuccessWithStore(store, project); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	shown = CheckAndShowWithStore(store, project, config.ProfileDeveloper, true, &buf)
	if shown {
		t.Fatal("step 4: unexpected hint after both exhausted")
	}

	// Verify final state
	state, _ := store.Load(project)
	if state.GoSuccessCount != 4 {
		t.Errorf("final GoSuccessCount = %d, want 4", state.GoSuccessCount)
	}
	if !state.FirstHintShown || !state.SecondHintShown {
		t.Errorf("final state: first=%v second=%v, want both true", state.FirstHintShown, state.SecondHintShown)
	}
}

func TestCheckAndShow_ReadOnlyStateDir(t *testing.T) {
	// Store pointing to nonexistent deeply nested path
	store := NewStateStoreWithPath("/nonexistent/deeply/nested/state.json")

	// checkAndShowWithStore should gracefully return false on Load failure
	var buf bytes.Buffer
	shown := checkAndShowWithStore(store, "/test/project", &buf)
	if shown {
		t.Error("hint shown with unreadable store, want graceful degradation")
	}
	if buf.Len() != 0 {
		t.Errorf("output = %q, want empty", buf.String())
	}
}
