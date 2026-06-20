package cli

import (
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/config"
)

type missingInheritedHarnessSections struct {
	reviewGate bool
	orchestra  bool
	quality    bool
	verify     bool
	design     bool
}

func (m missingInheritedHarnessSections) any() bool {
	return m.reviewGate || m.orchestra || m.quality || m.verify || m.design
}

type effectiveHarnessConfig struct {
	Config    *config.HarnessConfig
	ConfigDir string
	ParentDir string
	Missing   missingInheritedHarnessSections
}

func (e effectiveHarnessConfig) designRoot() string {
	if e.Missing.design && e.ParentDir != "" {
		return e.ParentDir
	}
	if e.ConfigDir != "" {
		return e.ConfigDir
	}
	return "."
}

func loadAndMigrateHarnessConfig(dir string) (*config.HarnessConfig, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}
	// Apply migrations in-memory so orchestra always uses current provider set.
	_, _ = config.MigrateOrchestraConfig(cfg)
	return cfg, nil
}

func detectMissingInheritedHarnessSections(dir string, cfg *config.HarnessConfig) (missingInheritedHarnessSections, error) {
	missing := missingInheritedHarnessSections{
		reviewGate: !hasReviewProviderWiring(cfg),
		orchestra:  cfg == nil || (len(cfg.Orchestra.Providers) == 0 && len(cfg.Orchestra.Commands) == 0),
		quality:    cfg == nil || cfg.Quality.Default == "" || len(cfg.Quality.Presets) == 0,
	}

	verifyMissing, err := config.MissingTopLevelKey(dir, "verify")
	if err != nil {
		return missingInheritedHarnessSections{}, err
	}
	missing.verify = verifyMissing
	designMissing, err := config.MissingTopLevelKey(dir, "design")
	if err != nil {
		return missingInheritedHarnessSections{}, err
	}
	missing.design = designMissing
	return missing, nil
}

func findAncestorHarnessConfigWithReusableWiring(startDir string) (*config.HarnessConfig, string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, "", err
	}

	current := filepath.Clean(abs)
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return nil, "", nil
		}

		if _, statErr := os.Stat(filepath.Join(parent, "autopus.yaml")); statErr == nil {
			cfg, loadErr := loadAndMigrateHarnessConfig(parent)
			if loadErr != nil {
				return nil, "", loadErr
			}
			if hasReusableHarnessWiring(cfg) {
				return cfg, parent, nil
			}
		} else if !os.IsNotExist(statErr) {
			return nil, "", statErr
		}

		current = parent
	}
}

func hasReviewProviderWiring(cfg *config.HarnessConfig) bool {
	if cfg == nil {
		return false
	}
	return len(cfg.Spec.ReviewGate.Providers) > 0
}

func hasReusableHarnessWiring(cfg *config.HarnessConfig) bool {
	if cfg == nil {
		return false
	}
	return hasReviewProviderWiring(cfg) ||
		len(cfg.Orchestra.Providers) > 0 ||
		len(cfg.Orchestra.Commands) > 0 ||
		cfg.Quality.Default != "" ||
		len(cfg.Quality.Presets) > 0 ||
		cfg.Verify.Enabled ||
		cfg.Verify.DefaultViewport != "" ||
		cfg.Verify.MaxFixAttempts > 0 ||
		cfg.Design.Enabled ||
		len(cfg.Design.Paths) > 0 ||
		len(cfg.Design.UIFileGlobs) > 0
}

func inheritReusableHarnessWiring(child, parent *config.HarnessConfig, missing missingInheritedHarnessSections) {
	if child == nil || parent == nil {
		return
	}
	if missing.reviewGate && len(child.Spec.ReviewGate.Providers) == 0 && child.Spec.ReviewGate.Strategy == "" {
		child.Spec.ReviewGate = parent.Spec.ReviewGate
	}
	if missing.orchestra && len(child.Orchestra.Providers) == 0 && len(child.Orchestra.Commands) == 0 {
		child.Orchestra = parent.Orchestra
	}
	if missing.quality && child.Quality.Default == "" && len(child.Quality.Presets) == 0 {
		child.Quality = parent.Quality
	}
	if missing.verify {
		child.Verify = parent.Verify
	}
	if missing.design {
		child.Design = parent.Design
	}
}
