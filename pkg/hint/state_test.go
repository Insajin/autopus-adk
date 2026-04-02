package hint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStateStore(t *testing.T) {
	store, err := NewStateStore()
	if err != nil {
		t.Fatalf("NewStateStore() error: %v", err)
	}
	if store == nil {
		t.Fatal("NewStateStore() returned nil")
	}
	if store.path == "" {
		t.Fatal("NewStateStore() path is empty")
	}
}

func TestStateStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewStateStoreWithPath(path)

	state, err := store.Load("/some/project")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if state.GoSuccessCount != 0 {
		t.Errorf("GoSuccessCount = %d, want 0", state.GoSuccessCount)
	}
	if state.FirstHintShown {
		t.Error("FirstHintShown = true, want false")
	}
	if state.SecondHintShown {
		t.Error("SecondHintShown = true, want false")
	}
}

func TestStateStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewStateStoreWithPath(path)

	projectPath := "/my/project"
	want := &ProjectState{
		GoSuccessCount:  5,
		FirstHintShown:  true,
		SecondHintShown: false,
	}

	if err := store.Save(projectPath, want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load(projectPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.GoSuccessCount != want.GoSuccessCount {
		t.Errorf("GoSuccessCount = %d, want %d", got.GoSuccessCount, want.GoSuccessCount)
	}
	if got.FirstHintShown != want.FirstHintShown {
		t.Errorf("FirstHintShown = %v, want %v", got.FirstHintShown, want.FirstHintShown)
	}
	if got.SecondHintShown != want.SecondHintShown {
		t.Errorf("SecondHintShown = %v, want %v", got.SecondHintShown, want.SecondHintShown)
	}
}

func TestStateStore_MultipleProjects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	store := NewStateStoreWithPath(path)

	stateA := &ProjectState{GoSuccessCount: 3, FirstHintShown: true}
	stateB := &ProjectState{GoSuccessCount: 7, SecondHintShown: true}

	if err := store.Save("/project/a", stateA); err != nil {
		t.Fatalf("Save(a) error: %v", err)
	}
	if err := store.Save("/project/b", stateB); err != nil {
		t.Fatalf("Save(b) error: %v", err)
	}

	gotA, err := store.Load("/project/a")
	if err != nil {
		t.Fatalf("Load(a) error: %v", err)
	}
	gotB, err := store.Load("/project/b")
	if err != nil {
		t.Fatalf("Load(b) error: %v", err)
	}

	if gotA.GoSuccessCount != 3 {
		t.Errorf("project a: GoSuccessCount = %d, want 3", gotA.GoSuccessCount)
	}
	if gotA.FirstHintShown != true {
		t.Error("project a: FirstHintShown = false, want true")
	}
	if gotB.GoSuccessCount != 7 {
		t.Errorf("project b: GoSuccessCount = %d, want 7", gotB.GoSuccessCount)
	}
	if gotB.SecondHintShown != true {
		t.Error("project b: SecondHintShown = false, want true")
	}
}

func TestProjectKey(t *testing.T) {
	keyA := projectKey("/project/a")
	keyB := projectKey("/project/b")

	if keyA == "" {
		t.Fatal("projectKey returned empty string")
	}
	if len(keyA) != 32 {
		t.Errorf("projectKey length = %d, want 32", len(keyA))
	}
	if keyA == keyB {
		t.Error("different paths produced the same key")
	}

	// Verify determinism
	keyA2 := projectKey("/project/a")
	if keyA != keyA2 {
		t.Error("projectKey is not deterministic")
	}
}

func TestStateStore_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Write corrupt JSON
	if err := os.WriteFile(path, []byte("{corrupt!!!"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	store := NewStateStoreWithPath(path)

	// Load should gracefully degrade
	state, err := store.Load("/some/project")
	if err != nil {
		t.Fatalf("Load() error on corrupt file: %v", err)
	}
	if state.GoSuccessCount != 0 {
		t.Errorf("GoSuccessCount = %d, want 0", state.GoSuccessCount)
	}

	// Save should overwrite corrupt file
	newState := &ProjectState{GoSuccessCount: 1}
	if err := store.Save("/some/project", newState); err != nil {
		t.Fatalf("Save() error on corrupt file: %v", err)
	}

	got, err := store.Load("/some/project")
	if err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}
	if got.GoSuccessCount != 1 {
		t.Errorf("GoSuccessCount = %d, want 1", got.GoSuccessCount)
	}
}

func TestStateStore_SaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "state.json")

	// Ensure parent dir exists for the test
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	store := NewStateStoreWithPath(path)

	// File should not exist yet
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("state file already exists before save")
	}

	state := &ProjectState{GoSuccessCount: 2, FirstHintShown: true}
	if err := store.Save("/new/project", state); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// File should now exist
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file does not exist after save: %v", err)
	}

	// Verify content
	got, err := store.Load("/new/project")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.GoSuccessCount != 2 {
		t.Errorf("GoSuccessCount = %d, want 2", got.GoSuccessCount)
	}
}
