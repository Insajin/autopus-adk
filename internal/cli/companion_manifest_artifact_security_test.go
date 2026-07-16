package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDigestRegularFile_PostReadMutationIsRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{
			name: "path replacement",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("replacement"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "in-place content change",
			mutate: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("changed-in-place"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact")
			if err := os.WriteFile(path, []byte("original-artifact"), 0o600); err != nil {
				t.Fatal(err)
			}

			digest, err := digestRegularFileWithPostReadHook(path, func() {
				test.mutate(t, path)
			})
			if err == nil {
				t.Fatalf("digest = %q, error = nil, want mutation rejection", digest)
			}
		})
	}
}
