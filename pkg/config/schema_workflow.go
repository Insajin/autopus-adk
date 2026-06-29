package config

import "fmt"

// WorkflowConf holds the workflow configuration settings.
type WorkflowConf struct {
	TeamDefault       bool `yaml:"team_default"`
	CoverageThreshold int  `yaml:"coverage_threshold,omitempty"`
}

// Validate checks that the workflow configuration is valid.
// An out-of-range coverage threshold (outside 0..100) is rejected.
func (w WorkflowConf) Validate() error {
	if w.CoverageThreshold < 0 || w.CoverageThreshold > 100 {
		return fmt.Errorf("workflow: coverage_threshold %d must be between 0 and 100", w.CoverageThreshold)
	}
	return nil
}
