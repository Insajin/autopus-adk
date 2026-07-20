package evidence

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestReadDesktopObservationArtifact_RejectsDescriptorSwap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-observation.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"original":true}`), 0o600))
	ops := desktopObservationFileOps{
		lstat: os.Lstat,
		open: func(name string) (*os.File, error) {
			if err := os.Rename(name, name+".original"); err != nil {
				return nil, err
			}
			if err := os.WriteFile(name, []byte(`{"replacement":true}`), 0o600); err != nil {
				return nil, err
			}
			return os.Open(name)
		},
	}

	_, err := readDesktopObservationArtifact(path, ops)

	require.Error(t, err)
	assert.ErrorContains(t, err, "identity")
}

func TestReadDesktopObservationArtifact_RejectsSymlinkSwapToSameIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-observation.json")
	target := filepath.Join(dir, "original.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"original":true}`), 0o600))
	ops := desktopObservationFileOps{
		lstat: os.Lstat,
		open: func(name string) (*os.File, error) {
			if err := os.Rename(name, target); err != nil {
				return nil, err
			}
			if err := os.Symlink(target, name); err != nil {
				return nil, err
			}
			return os.Open(name)
		},
	}

	_, err := readDesktopObservationArtifact(path, ops)

	require.Error(t, err)
}

func TestReadDesktopObservationArtifact_DoesNotOpenNonRegularInput(t *testing.T) {
	openCalls := 0
	ops := desktopObservationFileOps{
		lstat: func(string) (fs.FileInfo, error) { return staticFileInfo{mode: os.ModeNamedPipe}, nil },
		open: func(string) (*os.File, error) {
			openCalls++
			return nil, errors.New("must not open")
		},
	}

	_, err := readDesktopObservationArtifact("named-pipe", ops)

	require.Error(t, err)
	assert.Zero(t, openCalls)
}

func TestDesktopObservationArtifact_RejectsOversizeBeforePublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
	require.NoError(t, os.WriteFile(
		manifest.Artifacts[0].Path,
		bytes.Repeat([]byte("x"), desktopobserve.MaxEnvelopeBytes+1),
		0o600,
	))
	output := filepath.Join(dir, "published")

	_, err := WriteFinalManifest(manifest, output)

	require.Error(t, err)
	assert.NoDirExists(t, output)
}

func TestDesktopObservationArtifact_DoesNotReadSymlinkTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
	target := manifest.Artifacts[0].Path
	link := filepath.Join(dir, "linked-observation.json")
	require.NoError(t, os.Symlink(target, link))
	manifest.Artifacts[0].Path = link
	output := filepath.Join(dir, "published")

	_, err := WriteFinalManifest(manifest, output)

	require.Error(t, err)
	assert.NoDirExists(t, output)
}

func TestDesktopObservationArtifact_PublishedNameIsFixed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := desktopObservationManifest(t, dir, successfulObservationEvidence(t), "passed")
	injectedName := filepath.Join(dir, "provider_dump.json")
	require.NoError(t, os.Rename(manifest.Artifacts[0].Path, injectedName))
	manifest.Artifacts[0].Path = injectedName

	manifestPath, err := WriteFinalManifest(manifest, filepath.Join(dir, "published"))

	require.NoError(t, err)
	loaded, err := LoadManifest(manifestPath)
	require.NoError(t, err)
	require.Len(t, loaded.Artifacts, 1)
	assert.Equal(t, "artifacts/desktop_observation/desktop-observation.json", loaded.Artifacts[0].Path)
	assert.NotContains(t, loaded.Artifacts[0].Path, "provider_dump")
}

type staticFileInfo struct {
	mode fs.FileMode
}

func (info staticFileInfo) Name() string       { return "named-pipe" }
func (info staticFileInfo) Size() int64        { return 0 }
func (info staticFileInfo) Mode() fs.FileMode  { return info.mode }
func (info staticFileInfo) ModTime() time.Time { return time.Time{} }
func (info staticFileInfo) IsDir() bool        { return false }
func (info staticFileInfo) Sys() any           { return nil }
