package spec_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// countTestDocLines mirrors the production line counter in quality_preflight.go
// so fixtures can be padded to an exact logical line count.
func countTestDocLines(content string) int {
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

// padToLines appends innocuous filler lines until content has exactly target
// logical lines, preserving every existing section and heading.
func padToLines(content string, target int) string {
	var b strings.Builder
	b.WriteString(content)
	current := countTestDocLines(content)
	if current > 0 && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	for i := current; i < target; i++ {
		fmt.Fprintf(&b, "filler content line %d\n", i)
	}
	return b.String()
}

func TestValidateSpecSet_WarnsOnOverCapPlanDoc(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"plan.md": padToLines(validPlanMD(), 250),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	requireNoErrorLevel(t, errs)
	requireWarningContains(t, errs, "plan.md", "plan.md", "250", "200")
}

func TestValidateSpecSet_WarnsOnOverCapResearchDoc(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"research.md": padToLines(validResearchMD(), 429),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	requireNoErrorLevel(t, errs)
	requireWarningContains(t, errs, "research.md", "research.md", "429", "200")
}

func TestValidateSpecSet_NoLineCapWarningUnderCap(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, nil)

	errs := spec.ValidateSpecSet(specDir, doc)
	for _, e := range errs {
		if e.Level == "warning" && strings.Contains(e.Message, "review injection cap") {
			t.Fatalf("unexpected line-cap warning for under-cap docs: %s", e.Message)
		}
	}
}

func requireNoErrorLevel(t *testing.T, errs []spec.ValidationError) {
	t.Helper()
	for _, e := range errs {
		if e.Level == "error" {
			t.Fatalf("unexpected error-level finding: %s: %s", e.Field, e.Message)
		}
	}
}

func requireWarningContains(t *testing.T, errs []spec.ValidationError, field string, substrs ...string) {
	t.Helper()
	for _, e := range errs {
		if e.Level != "warning" || e.Field != field {
			continue
		}
		matched := true
		for _, s := range substrs {
			if !strings.Contains(e.Message, s) {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("expected warning field=%s containing %v, got %#v", field, substrs, errs)
}
