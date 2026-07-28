package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDaemonExecutable(t *testing.T) {
	t.Parallel()

	releaseIdentity := daemonExecutableIdentity{
		PackagePath:   "github.com/insajin/autopus-adk/cmd/auto",
		ModulePath:    "github.com/insajin/autopus-adk",
		ModuleVersion: "(devel)",
		BuildVersion:  "0.50.90",
		BuildCommit:   "abc1234",
	}
	tests := []struct {
		name     string
		path     string
		identity daemonExecutableIdentity
		wantErr  bool
	}{
		{
			name:     "official release binary",
			path:     "/usr/local/bin/auto",
			identity: releaseIdentity,
		},
		{
			name: "official go install binary",
			path: "/home/example/go/bin/auto",
			identity: daemonExecutableIdentity{
				PackagePath:   "github.com/insajin/autopus-adk/cmd/auto",
				ModulePath:    "github.com/insajin/autopus-adk",
				ModuleVersion: "v0.50.90",
				BuildVersion:  "0.50.90",
				BuildCommit:   "none",
			},
		},
		{
			name: "renamed Go test binary",
			path: "/private/tmp/auto",
			identity: daemonExecutableIdentity{
				PackagePath:   "github.com/insajin/autopus-adk/pkg/worker/setup.test",
				ModulePath:    "github.com/insajin/autopus-adk",
				ModuleVersion: "(devel)",
				BuildVersion:  "dev",
				BuildCommit:   "abc1234",
			},
			wantErr: true,
		},
		{
			name:     "official build with wrong basename",
			path:     "/private/tmp/autopus-worker-123",
			identity: releaseIdentity,
			wantErr:  true,
		},
		{
			name: "development build",
			path: "/usr/local/bin/auto",
			identity: daemonExecutableIdentity{
				PackagePath:   "github.com/insajin/autopus-adk/cmd/auto",
				ModulePath:    "github.com/insajin/autopus-adk",
				ModuleVersion: "(devel)",
				BuildVersion:  "dev",
				BuildCommit:   "abc1234",
			},
			wantErr: true,
		},
		{
			name: "dirty build",
			path: "/usr/local/bin/auto",
			identity: daemonExecutableIdentity{
				PackagePath:   "github.com/insajin/autopus-adk/cmd/auto",
				ModulePath:    "github.com/insajin/autopus-adk",
				ModuleVersion: "(devel)",
				BuildVersion:  "0.50.90-dirty",
				BuildCommit:   "abc1234",
			},
			wantErr: true,
		},
		{
			name: "release version without build provenance",
			path: "/usr/local/bin/auto",
			identity: daemonExecutableIdentity{
				PackagePath:   "github.com/insajin/autopus-adk/cmd/auto",
				ModulePath:    "github.com/insajin/autopus-adk",
				ModuleVersion: "(devel)",
				BuildVersion:  "0.50.90",
				BuildCommit:   "none",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDaemonExecutableIdentity(tt.path, tt.identity)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "refusing")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInstallAndStartDaemon_RenamedGoTestBinaryFailsClosed(t *testing.T) {
	const helperEnv = "AUTOPUS_RENAMED_TEST_BINARY_HELPER"
	if os.Getenv(helperEnv) == "1" {
		err := installAndStartDaemon()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "untrusted auto build identity")
		return
	}

	binaryPath := filepath.Join(t.TempDir(), "auto")
	compile := exec.Command("go", "test", "-c", "-o", binaryPath, ".")
	compileOutput, err := compile.CombinedOutput()
	require.NoError(t, err, string(compileOutput))

	run := exec.Command(binaryPath, "-test.run=^TestInstallAndStartDaemon_RenamedGoTestBinaryFailsClosed$")
	run.Env = append(os.Environ(), helperEnv+"=1", "HOME="+t.TempDir())
	output, err := run.CombinedOutput()
	require.NoError(t, err, string(output))
}
