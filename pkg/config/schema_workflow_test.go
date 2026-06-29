package config

import (
	"strings"
	"testing"
)

func TestWorkflowConf_Validate(t *testing.T) {
	// Test valid configurations
	w1 := WorkflowConf{
		TeamDefault:       true,
		CoverageThreshold: 85,
	}
	if err := w1.Validate(); err != nil {
		t.Errorf("expected no error for valid coverage threshold, got %v", err)
	}

	w2 := WorkflowConf{
		TeamDefault:       false,
		CoverageThreshold: 0, // 0 is allowed as unset
	}
	if err := w2.Validate(); err != nil {
		t.Errorf("expected no error for 0 coverage threshold, got %v", err)
	}

	// Test invalid configurations
	w3 := WorkflowConf{
		TeamDefault:       true,
		CoverageThreshold: 150,
	}
	err := w3.Validate()
	if err == nil {
		t.Errorf("expected validation error for threshold 150, got nil")
	} else {
		msg := err.Error()
		if !strings.Contains(msg, "workflow") || !strings.Contains(msg, "coverage_threshold") {
			t.Errorf("expected error message to contain 'workflow' and 'coverage_threshold', got '%s'", msg)
		}
	}

	w4 := WorkflowConf{
		TeamDefault:       true,
		CoverageThreshold: -5,
	}
	if err := w4.Validate(); err == nil {
		t.Errorf("expected validation error for negative threshold, got nil")
	}
}
