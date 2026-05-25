package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

type journeyCoverage struct {
	lanes map[string]string
}

func loadJourneyCoverage(projectDir string) (journeyCoverage, []string) {
	coverage := journeyCoverage{lanes: map[string]string{}}
	pattern := filepath.Join(projectDir, ".autopus", "qa", "journeys", "*.yaml")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return coverage, []string{"could not inspect existing Journey Packs: " + err.Error()}
	}
	warnings := []string{}
	for _, path := range paths {
		pack, err := journey.LoadFile(path)
		if err != nil {
			warnings = append(warnings, existingPackWarning(projectDir, path, err))
			continue
		}
		if err := journey.Validate(pack, projectDir); err != nil {
			warnings = append(warnings, existingPackWarning(projectDir, path, err))
			continue
		}
		for _, lane := range pack.Lanes {
			lane = normalizeLane(lane)
			if lane == "" {
				continue
			}
			if coverage.lanes[lane] == "" {
				coverage.lanes[lane] = pack.ID
			}
		}
	}
	return coverage, warnings
}

func existingPackWarning(projectDir, path string, err error) string {
	rel, relErr := filepath.Rel(projectDir, path)
	if relErr != nil {
		rel = path
	}
	return fmt.Sprintf("existing Journey Pack ignored during qa init analysis: %s: %s", filepath.ToSlash(rel), err)
}

func (coverage journeyCoverage) coversStarter(starter starterFile) bool {
	if len(starter.Lanes) == 0 {
		return false
	}
	for _, lane := range starter.Lanes {
		if coverage.lanes[normalizeLane(lane)] == "" {
			return false
		}
	}
	return true
}

func (coverage journeyCoverage) skippedStarter(projectDir string, starter starterFile) FileResult {
	coveredBy := []string{}
	for _, lane := range starter.Lanes {
		if id := coverage.lanes[normalizeLane(lane)]; id != "" {
			coveredBy = append(coveredBy, id)
		}
	}
	return FileResult{
		ID:     starter.ID,
		Path:   filepath.ToSlash(filepath.Join(projectDir, filepath.FromSlash(starter.RelPath))),
		Reason: "existing Journey Pack covers lane: " + strings.Join(uniqueStrings(coveredBy), ", "),
	}
}

func normalizeLane(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
