package selfupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestArchiveName verifies that the correct archive filename is generated for
// each supported GOOS/GOARCH combination.
func TestArchiveName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		goarch  string
		version string
		want    string
	}{
		{
			name:    "darwin arm64",
			goos:    "darwin",
			goarch:  "arm64",
			version: "0.7.0",
			want:    "autopus-adk_0.7.0_darwin_arm64.tar.gz",
		},
		{
			name:    "linux amd64",
			goos:    "linux",
			goarch:  "amd64",
			version: "0.7.0",
			want:    "autopus-adk_0.7.0_linux_amd64.tar.gz",
		},
		{
			name:    "darwin amd64",
			goos:    "darwin",
			goarch:  "amd64",
			version: "0.7.0",
			want:    "autopus-adk_0.7.0_darwin_amd64.tar.gz",
		},
		{
			name:    "linux arm64",
			goos:    "linux",
			goarch:  "arm64",
			version: "0.7.0",
			want:    "autopus-adk_0.7.0_linux_arm64.tar.gz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: ArchiveName is called with GOOS/GOARCH/version
			got := ArchiveName(tt.goos, tt.goarch, tt.version)

			// Then: the expected archive filename is returned
			assert.Equal(t, tt.want, got)
		})
	}
}
