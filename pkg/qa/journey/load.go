package journey

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadDir(projectDir string) ([]Pack, error) {
	pattern := filepath.Join(projectDir, ".autopus", "qa", "journeys", "*.yaml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	packs := make([]Pack, 0, len(paths))
	for _, path := range paths {
		pack, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		pack.Source = "configured"
		if err := Validate(pack, projectDir); err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

func LoadFile(path string) (Pack, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Pack{}, err
	}
	var pack Pack
	if err := yaml.Unmarshal(body, &pack); err != nil {
		return Pack{}, err
	}
	if strings.TrimSpace(pack.Command.CWD) == "" {
		pack.Command.CWD = "."
	}
	return pack, nil
}

func HasLane(pack Pack, lane string) bool {
	if strings.TrimSpace(lane) == "" {
		return true
	}
	for _, value := range pack.Lanes {
		if strings.EqualFold(strings.TrimSpace(value), lane) {
			return true
		}
	}
	return false
}
