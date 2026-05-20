package cli

import (
	"path/filepath"
	"sort"
	"strings"
)

func findOrphanedOrchestraProviderProcesses() []staleRuntimeProcess {
	rows, err := listRuntimeProcesses()
	if err != nil {
		return nil
	}

	var stale []staleRuntimeProcess
	for _, row := range rows {
		if row.PPID != 1 || !isOrchestraProviderCommand(row.Command) {
			continue
		}
		stale = append(stale, staleRuntimeProcess{
			PID:     row.PID,
			PPID:    row.PPID,
			Command: row.Command,
			Reason:  "orphaned orchestra provider process",
		})
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].PID < stale[j].PID })
	return stale
}

func isOrchestraProviderCommand(command string) bool {
	fields := strings.Fields(command)
	return commandHasProvider(fields, "agy") ||
		commandHasProviderHeadlessMode(fields, "gemini", "-p", "--prompt") ||
		commandHasProviderHeadlessMode(fields, "codex", "exec") ||
		commandHasProviderHeadlessMode(fields, "claude", "--print", "-p")
}

func commandHasProvider(fields []string, provider string) bool {
	for _, field := range fields {
		if filepath.Base(field) == provider {
			return true
		}
	}
	return false
}

func commandHasProviderHeadlessMode(fields []string, provider string, required ...string) bool {
	if !commandHasProvider(fields, provider) {
		return false
	}
	for _, want := range required {
		for _, field := range fields {
			if field == want {
				return true
			}
		}
	}
	return false
}
