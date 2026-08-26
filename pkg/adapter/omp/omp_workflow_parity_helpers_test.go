package omp

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/require"
)

func mappingTargetSet(files []adapter.FileMapping) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, file := range files {
		out[filepath.ToSlash(file.TargetPath)] = true
	}
	return out
}

func removeStableMapping(t *testing.T, root string, files []adapter.FileMapping) string {
	t.Helper()
	require.NotEmpty(t, files)
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.TargetPath)
	}
	sort.Strings(paths)
	require.NoError(t, os.Remove(filepath.Join(root, paths[0])))
	return filepath.ToSlash(paths[0])
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var out []byte
	for value > 0 {
		out = append([]byte{digits[value%10]}, out...)
		value /= 10
	}
	return string(out)
}
