package cli

import (
	"errors"
	"os"
	"strings"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: only task-owned paths and a closed loopback proxy may cross the child environment boundary.
var workflowContextManagedAllowedEnvironment = map[string]struct{}{
	"PI_CODING_AGENT_DIR": {}, "PI_CONFIG_FILES": {}, "HOME": {}, "TMPDIR": {},
	"XDG_CACHE_HOME": {}, "XDG_CONFIG_HOME": {}, "XDG_DATA_HOME": {}, "XDG_STATE_HOME": {},
	"PATH": {}, "NO_PROXY": {}, "no_proxy": {}, "HTTP_PROXY": {}, "HTTPS_PROXY": {}, "ALL_PROXY": {},
}

func validateWorkflowContextManagedEnvironmentIsolation(
	options WorkflowContextManagedRPCOptions, environment []string,
) error {
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		switch key {
		case "HOME", "TMPDIR", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME":
			canonical, err := canonicalWorkflowContextManagedPath(value, false)
			if err != nil || !workflowContextManagedPathBelow(options.RuntimeRoot, canonical) {
				return errors.New("managed OMP environment path is outside the task runtime")
			}
			info, statErr := os.Lstat(canonical)
			if statErr != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
				return errors.New("managed OMP environment path mode is invalid")
			}
		case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY":
			if value != "http://127.0.0.1:1" {
				return errors.New("managed OMP proxy must be the closed loopback sink")
			}
		case "NO_PROXY", "no_proxy":
			if value != "127.0.0.1,localhost,::1" {
				return errors.New("managed OMP no-proxy scope is invalid")
			}
		}
	}
	return nil
}
