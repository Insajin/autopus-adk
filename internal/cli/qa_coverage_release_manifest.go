package cli

import (
	"os"
	"path/filepath"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

func addReleaseJourneyCoverage(payload *qaCoveragePayload, row qarelease.LaneRow) {
	for _, manifestPath := range row.ManifestPaths {
		manifestPath = strings.TrimSpace(manifestPath)
		if manifestPath == "" {
			continue
		}
		manifest, err := qaevidence.LoadManifest(resolveCoverageManifestPath(payload.ProjectDir, manifestPath))
		journeyID := existingCoverageJourneyIDForManifest(payload.Journeys, manifestPath)
		if journeyID == "" {
			journeyID = fallbackCoverageJourneyID(manifestPath)
		}
		journey := qaCoverageJourney{
			JourneyID:    journeyID,
			Lane:         row.Lane,
			Status:       string(row.Status),
			ManifestPath: manifestPath,
			Source:       "release",
		}
		if err == nil {
			if strings.TrimSpace(manifest.ScenarioRef) != "" {
				journey.JourneyID = manifest.ScenarioRef
			}
			journey.Adapter = manifest.Runner.Name
			journey.Status = manifest.Status
		}
		payload.Journeys = upsertCoverageJourney(payload.Journeys, journey)
	}
}

func existingCoverageJourneyIDForManifest(journeys []qaCoverageJourney, manifestPath string) string {
	for _, journey := range journeys {
		if journey.ManifestPath == manifestPath && journey.JourneyID != "" {
			return journey.JourneyID
		}
	}
	return ""
}

func resolveCoverageManifestPath(projectDir, manifestPath string) string {
	if filepath.IsAbs(manifestPath) {
		return manifestPath
	}
	candidates := []string{filepath.Join(projectDir, manifestPath)}
	projectBase := filepath.Base(filepath.Clean(projectDir))
	cleanPath := filepath.Clean(manifestPath)
	if strings.HasPrefix(cleanPath, projectBase+string(os.PathSeparator)) {
		candidates = append(candidates, filepath.Join(filepath.Dir(projectDir), cleanPath))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func fallbackCoverageJourneyID(manifestPath string) string {
	parent := filepath.Base(filepath.Dir(manifestPath))
	if parent != "" && parent != "." && parent != string(os.PathSeparator) {
		return parent
	}
	base := strings.TrimSuffix(filepath.Base(manifestPath), filepath.Ext(manifestPath))
	if base == "" {
		return "manifest"
	}
	return base
}
