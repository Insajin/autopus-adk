package hint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ProjectState holds hint tracking state for a single project.
type ProjectState struct {
	GoSuccessCount  int  `json:"go_success_count"`
	FirstHintShown  bool `json:"first_hint_shown"`
	SecondHintShown bool `json:"second_hint_shown"`
}

// stateFile is the top-level structure of ~/.autopus/state.json.
type stateFile struct {
	Version  int                     `json:"version"`
	Projects map[string]ProjectState `json:"projects"`
}

// StateStore manages ~/.autopus/state.json.
type StateStore struct {
	path string
	mu   sync.Mutex
}

// NewStateStore creates a store using ~/.autopus/state.json.
// Returns error if home directory cannot be determined.
func NewStateStore() (*StateStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".autopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &StateStore{path: filepath.Join(dir, "state.json")}, nil
}

// NewStateStoreWithPath creates a store with a custom path (for testing).
func NewStateStoreWithPath(path string) *StateStore {
	return &StateStore{path: path}
}

// Load reads the project state for the given project path.
// Returns zero-value state if file doesn't exist or is unreadable.
func (s *StateStore) Load(projectPath string) (*ProjectState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.readFile()
	if err != nil {
		// Graceful degradation: return empty state
		return &ProjectState{}, nil
	}

	key := projectKey(projectPath)
	if ps, ok := sf.Projects[key]; ok {
		return &ps, nil
	}
	return &ProjectState{}, nil
}

// Save writes the project state for the given project path.
func (s *StateStore) Save(projectPath string, state *ProjectState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sf, err := s.readFile()
	if err != nil {
		sf = &stateFile{Version: 1, Projects: make(map[string]ProjectState)}
	}

	key := projectKey(projectPath)
	sf.Projects[key] = *state

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// readFile reads and parses the state file.
func (s *StateStore) readFile() (*stateFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}
	if sf.Projects == nil {
		sf.Projects = make(map[string]ProjectState)
	}
	return &sf, nil
}

// projectKey computes a stable key from the absolute project path.
// Uses first 16 bytes of SHA-256 (32-char hex) for sufficient uniqueness.
func projectKey(projectPath string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	h := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(h[:16])
}
