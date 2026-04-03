package budget

import (
	"bytes"
	"strings"
	"testing"
)

// REQ-BUDGET-06: 70% warning message injection.
func TestWarningInjector_WarnLevel(t *testing.T) {
	var buf bytes.Buffer
	wi := NewWarningInjector(&buf)

	wi.Inject(IncrementResult{
		Count:   70,
		Level:   LevelWarn,
		Changed: true,
		Budget:  DefaultBudget(100),
	})

	got := buf.String()
	if !strings.Contains(got, "BUDGET WARNING") {
		t.Errorf("expected warning message, got: %s", got)
	}
	if !strings.Contains(got, "70/100") {
		t.Errorf("expected count in message, got: %s", got)
	}
}

// REQ-BUDGET-07: 90% danger message injection.
func TestWarningInjector_DangerLevel(t *testing.T) {
	var buf bytes.Buffer
	wi := NewWarningInjector(&buf)

	wi.Inject(IncrementResult{
		Count:   90,
		Level:   LevelDanger,
		Changed: true,
		Budget:  DefaultBudget(100),
	})

	got := buf.String()
	if !strings.Contains(got, "BUDGET CRITICAL") {
		t.Errorf("expected critical message, got: %s", got)
	}
	if !strings.Contains(got, "90/100") {
		t.Errorf("expected count in message, got: %s", got)
	}
}

// REQ-BUDGET-06/07: No message when level hasn't changed.
func TestWarningInjector_NoChangeNoMessage(t *testing.T) {
	var buf bytes.Buffer
	wi := NewWarningInjector(&buf)

	wi.Inject(IncrementResult{
		Count:   71,
		Level:   LevelWarn,
		Changed: false,
		Budget:  DefaultBudget(100),
	})

	if buf.Len() != 0 {
		t.Errorf("expected no output when Changed=false, got: %s", buf.String())
	}
}

// No message for LevelOK.
func TestWarningInjector_OKLevel(t *testing.T) {
	var buf bytes.Buffer
	wi := NewWarningInjector(&buf)

	wi.Inject(IncrementResult{
		Count:   5,
		Level:   LevelOK,
		Changed: true,
		Budget:  DefaultBudget(100),
	})

	if buf.Len() != 0 {
		t.Errorf("expected no output for LevelOK, got: %s", buf.String())
	}
}

// No message for LevelExhausted (emergency stop handles it).
func TestWarningInjector_ExhaustedLevel(t *testing.T) {
	var buf bytes.Buffer
	wi := NewWarningInjector(&buf)

	wi.Inject(IncrementResult{
		Count:   100,
		Level:   LevelExhausted,
		Changed: true,
		Budget:  DefaultBudget(100),
	})

	if buf.Len() != 0 {
		t.Errorf("expected no output for LevelExhausted, got: %s", buf.String())
	}
}
