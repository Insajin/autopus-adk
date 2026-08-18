package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseOptions_RejectsMissingAndTrailingInputs(t *testing.T) {
	if _, err := parseOptions(nil); err == nil {
		t.Fatal("missing arguments passed")
	}
	arguments := []string{
		"--lineage", "lineage", "--signature", "signature", "--receipt-bundle", "bundle",
		"--key-id", "key", "--handoff", "v1", "--minimum-rollback-floor", "5093",
		"--upstream-sha256", "sha256:" + strings.Repeat("1", 64),
		"--executable-sha256", "sha256:" + strings.Repeat("2", 64),
		"--source-repository", "Insajin/autopus-adk", "--source-commit", strings.Repeat("3", 40),
		"--source-tree", strings.Repeat("4", 40), "--target", "darwin-arm64", "--version", "0.50.107",
	}
	if _, err := parseOptions(append(arguments, "trailing")); err == nil {
		t.Fatal("trailing argument passed")
	}
	if err := run(arguments, time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("missing files established release lineage")
	}
}

func TestRun_RejectsUnsafeLineageAtUserVisibleReadBoundary(t *testing.T) {
	dir := t.TempDir()
	signature := filepath.Join(dir, "lineage.sig")
	if err := os.WriteFile(signature, make([]byte, 64), 0o600); err != nil {
		t.Fatalf("write signature fixture: %v", err)
	}
	arguments := []string{
		"--lineage", dir, "--signature", signature, "--receipt-bundle", filepath.Join(dir, "bundle"),
		"--key-id", "key", "--handoff", "v1", "--minimum-rollback-floor", "5093",
		"--upstream-sha256", "sha256:" + strings.Repeat("1", 64),
		"--executable-sha256", "sha256:" + strings.Repeat("2", 64),
		"--source-repository", "Insajin/autopus-adk", "--source-commit", strings.Repeat("3", 40),
		"--source-tree", strings.Repeat("4", 40), "--target", "darwin-arm64", "--version", "v0.50.107",
	}
	err := run(arguments, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC))
	if err == nil || err.Error() != "lineage input is unsafe" {
		t.Fatalf("unsafe lineage read result = %v", err)
	}
}
