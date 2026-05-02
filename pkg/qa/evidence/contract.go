package evidence

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateSurfaceContract(manifest Manifest) error {
	switch manifest.Surface {
	case "browser":
		if manifest.Lane == "golden" {
			return validateBrowserGoldenContract(manifest)
		}
	case "desktop":
		return validateDesktopContract(manifest)
	}
	return nil
}

func validateBrowserGoldenContract(manifest Manifest) error {
	if manifest.OracleResults.A11y == nil {
		return fmt.Errorf("browser golden evidence requires oracle_results.a11y")
	}
	if manifest.ReproductionCommand == "" && manifest.Runner.Command == "" {
		return fmt.Errorf("browser golden evidence requires a reproduction command")
	}
	byKind := artifactsByKind(manifest.Artifacts)
	if err := requireEitherArtifact(byKind, "trace_summary", "trace_sanitized"); err != nil {
		return err
	}
	if err := requireEitherArtifact(byKind, "screenshot_masked", "screenshot_quarantined"); err != nil {
		return err
	}
	for _, kind := range []string{"console", "network_summary", "a11y_snapshot", "oracle_summary"} {
		if len(byKind[kind]) == 0 {
			return fmt.Errorf("browser golden evidence missing artifact kind %s", kind)
		}
	}
	if artifacts := byKind["trace_summary"]; len(artifacts) > 0 && !allArtifactsHaveSuffix(artifacts, ".json") {
		return fmt.Errorf("trace_summary artifact must end with .json")
	}
	if artifacts := byKind["trace_sanitized"]; len(artifacts) > 0 && !allArtifactsHaveSuffix(artifacts, ".zip") {
		return fmt.Errorf("trace_sanitized artifact must end with .zip")
	}
	if artifacts := byKind["a11y_snapshot"]; len(artifacts) > 0 && !allArtifactsHaveAnySuffix(artifacts, ".aria.yml", ".json") {
		return fmt.Errorf("a11y_snapshot artifact must end with .aria.yml or .json")
	}
	return nil
}

func validateDesktopContract(manifest Manifest) error {
	if manifest.SourceRefs.SourceSpec != "SPEC-DESKTOP-017" {
		return fmt.Errorf("desktop evidence source_refs.source_spec must be SPEC-DESKTOP-017")
	}
	if manifest.OracleResults.Desktop == nil {
		return fmt.Errorf("desktop evidence requires oracle_results.desktop")
	}
	if strings.TrimSpace(manifest.OracleResults.Desktop.TimeoutClassification) == "" {
		return fmt.Errorf("desktop evidence requires timeout_classification")
	}
	byKind := artifactsByKind(manifest.Artifacts)
	for _, kind := range []string{"screenshot", "app_log", "driver_log", "command_output"} {
		if len(byKind[kind]) == 0 {
			return fmt.Errorf("desktop evidence missing artifact kind %s", kind)
		}
	}
	return nil
}

func artifactsByKind(artifacts []ArtifactRef) map[string][]ArtifactRef {
	byKind := make(map[string][]ArtifactRef)
	for _, artifact := range artifacts {
		kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
		byKind[kind] = append(byKind[kind], artifact)
	}
	return byKind
}

func requireEitherArtifact(byKind map[string][]ArtifactRef, left, right string) error {
	if len(byKind[left]) == 0 && len(byKind[right]) == 0 {
		return fmt.Errorf("evidence missing artifact kind %s or %s", left, right)
	}
	return nil
}

func allArtifactsHaveSuffix(artifacts []ArtifactRef, suffix string) bool {
	return allArtifactsHaveAnySuffix(artifacts, suffix)
}

func allArtifactsHaveAnySuffix(artifacts []ArtifactRef, suffixes ...string) bool {
	for _, artifact := range artifacts {
		name := strings.ToLower(filepath.ToSlash(artifact.Path))
		matched := false
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
