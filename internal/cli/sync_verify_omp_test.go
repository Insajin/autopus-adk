package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncVerifyBlocksOMPGeneratedPathsInBothTopologies(t *testing.T) {
	for _, multi := range []bool{false, true} {
		name := "single"
		if multi {
			name = "multi"
		}
		t.Run(name, func(t *testing.T) {
			root := singleAutopusRepo(t)
			owner := root
			if multi {
				owner = nestedRepo(t, root, "module")
			}
			for _, path := range []string{".omp/skills/review/SKILL.md", ".omp/commands/auto.md", ".autopus/claude-code-permissions.json"} {
				syncWrite(t, owner, path, "generated\n")
			}
			var out bytes.Buffer
			_, err := executeSyncVerify(&out, root, "", true)
			require.ErrorIs(t, err, errSyncVerifyStrict)
			plan := strings.Split(out.String(), "\nWarnings")[0]
			assert.NotContains(t, plan, "git -C")
			assert.Contains(t, out.String(), "blocked-path")
			assert.Contains(t, out.String(), ".omp/skills/review/SKILL.md excluded (generated/runtime)")
			assert.Contains(t, out.String(), ".autopus/claude-code-permissions.json excluded (generated/runtime)")
		})
	}
}
