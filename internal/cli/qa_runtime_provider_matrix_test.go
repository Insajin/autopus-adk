package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQARuntimeProvider_AllFourSurfacesRejectInvalidDuplicateAndConflictWithoutSideEffects(t *testing.T) {
	t.Parallel()

	surfaces := []string{"run", "release", "release-readiness", "full"}
	cases := []struct {
		name      string
		providers []string
		wantCode  string
	}{
		{name: "invalid", providers: []string{"automatic"}, wantCode: "qa_runtime_provider_invalid"},
		{name: "duplicate", providers: []string{"local", "local"}, wantCode: "qa_runtime_provider_conflict"},
		{name: "conflict", providers: []string{"local", "orca"}, wantCode: "qa_runtime_provider_conflict"},
	}

	for _, surface := range surfaces {
		surface := surface
		for _, test := range cases {
			test := test
			t.Run(surface+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				dir := writeCLIDesktopObservationProject(t)
				before := snapshotCLIProjectFiles(t, dir)
				output := filepath.Join(dir, "qa-output")
				command := newQACmd()
				var stdout bytes.Buffer
				command.SetOut(&stdout)
				command.SetErr(&bytes.Buffer{})
				command.SetArgs(runtimeProviderSurfaceArgs(surface, dir, output, test.providers))

				err := command.Execute()
				require.Error(t, err)
				payload := decodeJSONMap(t, stdout.Bytes())
				assert.Equal(t, test.wantCode, payload["error"].(map[string]any)["code"])
				assert.Equal(t, before, snapshotCLIProjectFiles(t, dir))
				assert.NoDirExists(t, output)
				assert.NoDirExists(t, filepath.Join(dir, ".autopus", "qa", "_release_readiness"))
			})
		}
	}
}

func runtimeProviderSurfaceArgs(surface, dir, output string, providers []string) []string {
	var args []string
	switch surface {
	case "run":
		args = []string{"run", "--project-dir", dir, "--lane", "desktop-native", "--journey", "desktop-accessibility-observe", "--adapter", "desktop-accessibility-observe", "--output", output, "--dry-run", "--format", "json"}
	case "release":
		args = []string{"release", "--project-dir", dir, "--output", output, "--dry-run", "--format", "json"}
	case "release-readiness":
		args = []string{"release-readiness", "--project-dir", dir, "--format", "json"}
	case "full":
		args = []string{"full", "--project-dir", dir, "--output", output, "--format", "json"}
	}
	for _, provider := range providers {
		args = append(args, "--runtime-provider", provider)
	}
	return args
}

func snapshotCLIProjectFiles(t *testing.T, root string) []string {
	t.Helper()
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			paths = append(paths, relative+"/")
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		paths = append(paths, fmt.Sprintf("%s:%x", relative, sha256.Sum256(body)))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(paths)
	return paths
}

func assertRuntimeProviderFlagOnce(t *testing.T, command, provider string) {
	t.Helper()
	assert.Equal(t, 1, strings.Count(command, "--runtime-provider"), command)
	tokens := strings.Fields(command)
	matched := 0
	for index, token := range tokens {
		if token == "--runtime-provider" {
			matched++
			require.Less(t, index+1, len(tokens))
			assert.Equal(t, provider, tokens[index+1])
		}
	}
	assert.Equal(t, 1, matched, command)
}
