package scenario

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DirRel is the project-local directory scenarios are authored in.
var DirRel = filepath.Join(".autopus", "qa", "scenarios")

// Dir returns the scenario directory for a project.
func Dir(projectDir string) string {
	return filepath.Join(projectDir, DirRel)
}

// LoadDir reads every scenario in deterministic order. It fails closed: one
// invalid scenario stops the whole compile, because a partially compiled set
// would silently ship fewer assertions than the project declared.
func LoadDir(projectDir string) ([]Scenario, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(projectDir), "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]Scenario, 0, len(paths))
	ids := map[string]string{}
	for _, path := range paths {
		loaded, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		if prior, clash := ids[loaded.ID]; clash {
			return nil, invalid(path, "qa_scenario_id_duplicate", "id %q already declared by %s", loaded.ID, prior)
		}
		ids[loaded.ID] = filepath.Base(path)
		out = append(out, loaded)
	}
	return out, nil
}

// LoadFile decodes one scenario with unknown keys rejected. A silently ignored
// key is how a misspelled assertion becomes an always-green test.
func LoadFile(path string) (Scenario, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var loaded Scenario
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&loaded); err != nil {
		return Scenario{}, invalid(filepath.Base(path), "qa_scenario_parse_invalid", "%s", err.Error())
	}
	loaded.Path = filepath.Base(path)
	loaded.ID = strings.TrimSpace(loaded.ID)
	loaded.Journey = strings.TrimSpace(loaded.Journey)
	loaded.Origin = strings.TrimRight(strings.TrimSpace(loaded.Origin), "/")
	if err := Validate(loaded); err != nil {
		return Scenario{}, err
	}
	return loaded, nil
}
