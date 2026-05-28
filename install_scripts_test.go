package autopusadk_test

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsInstallerGitBashPathHintEscapesColon(t *testing.T) {
	scriptBytes, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatalf("read install.ps1: %v", err)
	}
	script := string(scriptBytes)

	if strings.Contains(script, "\"$bashPath:`$PATH\"") {
		t.Fatalf("Git Bash PATH hint uses $bashPath: in a double-quoted PowerShell string; use ${bashPath}: so PowerShell does not parse the colon as part of the variable reference")
	}
	if !strings.Contains(script, "\"${bashPath}:`$PATH\"") {
		t.Fatalf("Git Bash PATH hint should interpolate ${bashPath}: before escaped $PATH")
	}
}
