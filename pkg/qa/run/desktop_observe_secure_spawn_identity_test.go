package run

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopCodeIdentity_UsesExactEmbeddedCodeDirectoryBytes(t *testing.T) {
	t.Parallel()
	directory := make([]byte, 44)
	binary.BigEndian.PutUint32(directory[0:4], desktopCodeDirectory)
	binary.BigEndian.PutUint32(directory[4:8], uint32(len(directory)))
	directory[37] = 2
	blob := embeddedSignatureFixture(t, directory)

	identity, err := parseDesktopEmbeddedSignature(blob)
	require.NoError(t, err)
	require.Equal(t, uint8(1), identity.count)
	expected := sha256.Sum256(directory)
	assert.Equal(t, expected[:20], identity.digests[0][:])

	directory[43] = 1
	changed, err := parseDesktopEmbeddedSignature(embeddedSignatureFixture(t, directory))
	require.NoError(t, err)
	assert.NotEqual(t, identity, changed)
}

func TestDesktopCodeIdentity_RejectsMalformedOrUnsupportedSignatures(t *testing.T) {
	t.Parallel()
	for name, blob := range map[string][]byte{
		"empty":       nil,
		"wrong magic": make([]byte, 20),
		"truncated":   {0xfa, 0xde, 0x0c, 0xc0, 0, 0, 0, 20, 0, 0, 0, 1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseDesktopEmbeddedSignature(blob)
			assert.ErrorIs(t, err, errDesktopProviderUnavailable)
		})
	}
	directory := make([]byte, 44)
	binary.BigEndian.PutUint32(directory[0:4], desktopCodeDirectory)
	binary.BigEndian.PutUint32(directory[4:8], uint32(len(directory)))
	directory[37] = 99
	_, err := parseDesktopEmbeddedSignature(embeddedSignatureFixture(t, directory))
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
}

func TestDesktopCodeIdentity_BoundsCodeDirectoriesNotUnrelatedSignatureSlots(t *testing.T) {
	t.Parallel()
	directory := make([]byte, 44)
	binary.BigEndian.PutUint32(directory[0:4], desktopCodeDirectory)
	binary.BigEndian.PutUint32(directory[4:8], uint32(len(directory)))
	directory[37] = 2
	const count = 8
	const directoryOffset = 12 + count*8
	blob := make([]byte, directoryOffset+len(directory))
	binary.BigEndian.PutUint32(blob[0:4], desktopEmbeddedSignature)
	binary.BigEndian.PutUint32(blob[4:8], uint32(len(blob)))
	binary.BigEndian.PutUint32(blob[8:12], count)
	binary.BigEndian.PutUint32(blob[12:16], 0)
	binary.BigEndian.PutUint32(blob[16:20], directoryOffset)
	for index := 1; index < count; index++ {
		binary.BigEndian.PutUint32(blob[12+index*8:16+index*8], uint32(index))
	}
	copy(blob[directoryOffset:], directory)

	identity, err := parseDesktopEmbeddedSignature(blob)
	require.NoError(t, err)
	assert.Equal(t, uint8(1), identity.count)
}

func TestDesktopCodeIdentity_ConfiguredSignedArtifactMatchesCodesignStructure(t *testing.T) {
	artifact := os.Getenv(desktopSignedArtifactEnvironment)
	if artifact == "" {
		t.Skip("release-signed artifact is not configured")
	}
	executable := filepath.Join(artifact, "Contents", "MacOS", desktopProviderExecutableName)
	info, _, identity, err := snapshotDesktopExecutable(context.Background(), executable, -1)
	require.NoError(t, err)
	assert.Positive(t, info.Size())
	assert.NotZero(t, identity.count)
}

func TestSecureDesktopSpawn_ConfiguredSignedArtifactRunsBoundImage(t *testing.T) {
	artifact := os.Getenv(desktopSignedArtifactEnvironment)
	if artifact == "" || !secureDesktopSpawnSupported() {
		t.Skip("secure Darwin spawn and release-signed artifact are required")
	}
	resolver := &processDesktopProviderResolver{artifactPath: artifact}
	client, err := resolver.ResolveLocal(context.Background())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	identity, err := client.Handshake(ctx)
	require.NoError(t, err)
	assert.Equal(t, "autopus-desktop-local", identity.Name)
	assert.Equal(t, 1, identity.ProtocolVersion)
}

func embeddedSignatureFixture(t *testing.T, directory []byte) []byte {
	t.Helper()
	const directoryOffset = 20
	blob := bytes.Repeat([]byte{0}, directoryOffset+len(directory))
	binary.BigEndian.PutUint32(blob[0:4], desktopEmbeddedSignature)
	binary.BigEndian.PutUint32(blob[4:8], uint32(len(blob)))
	binary.BigEndian.PutUint32(blob[8:12], 1)
	binary.BigEndian.PutUint32(blob[12:16], 0)
	binary.BigEndian.PutUint32(blob[16:20], directoryOffset)
	copy(blob[directoryOffset:], directory)
	return blob
}
