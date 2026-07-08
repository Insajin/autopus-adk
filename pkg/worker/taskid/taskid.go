package taskid

import (
	"fmt"
	"strings"
)

const maxLength = 128

// Validate rejects task IDs that are unsafe for branch names, paths, logs, and cache filenames.
func Validate(id string) error {
	if id == "" {
		return fmt.Errorf("missing task ID")
	}
	if id != strings.TrimSpace(id) {
		return fmt.Errorf("invalid task ID %q: must not contain leading or trailing whitespace", id)
	}
	if len(id) > maxLength {
		return fmt.Errorf("invalid task ID %q: exceeds %d characters", id, maxLength)
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("invalid task ID %q: contains unsafe character %q", id, r)
	}
	return nil
}
