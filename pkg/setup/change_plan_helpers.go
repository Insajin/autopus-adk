package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func detectChangeAction(path string, content []byte, reasons changeReasonSet) (ChangeAction, string, error) {
	current, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(current, content):
		return ChangeActionPreserve, reasons.preserve, nil
	case err == nil:
		return ChangeActionUpdate, reasons.update, nil
	case os.IsNotExist(err):
		return ChangeActionCreate, reasons.create, nil
	default:
		return "", "", err
	}
}

func displayChangePath(projectDir string, absPath string) string {
	if rel, err := filepath.Rel(projectDir, absPath); err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.Clean(absPath)
}

func fingerprintPlan(mode ChangePlanMode, targets []plannedTarget) string {
	var builder strings.Builder
	builder.WriteString(string(mode))
	for _, target := range targets {
		builder.WriteString("|")
		builder.WriteString(string(target.change.Action))
		builder.WriteString("|")
		builder.WriteString(string(target.change.Class))
		builder.WriteString("|")
		builder.WriteString(target.change.Path)
		builder.WriteString("|")
		builder.WriteString(hashBytes(target.content))
	}
	return hashString(builder.String())
}

func defaultPlanReason(mode ChangePlanMode, fullRegeneration bool, note string) string {
	if note != "" {
		return note
	}
	if mode == ChangePlanModeGenerate {
		return "preview of tracked docs, generated surface, and runtime metadata"
	}
	if fullRegeneration {
		return "preview of a full regeneration for setup update"
	}
	return "preview of incremental setup update changes"
}
