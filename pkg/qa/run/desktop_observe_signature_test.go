package run

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopReleaseIdentity_RequiresPinnedDeveloperIDChainRuntimeAndArchitectures(t *testing.T) {
	t.Parallel()
	calls := make([][]string, 0, 2)
	runner := func(_ context.Context, arguments ...string) (string, error) {
		calls = append(calls, append([]string(nil), arguments...))
		if arguments[0] == "--verify" {
			return "", nil
		}
		return desktopCodesignFixtureMetadata(), nil
	}
	require.NoError(t, verifyDesktopReleaseArtifactIdentityWithRunner(
		context.Background(), "/release/Autopus Desktop.app",
		desktopReleaseBundleIdentifier, desktopReleaseTeamIdentifier, runner,
	))
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"--verify", "--deep", "--strict", "--all-architectures"},
		calls[0][:4])
	require.Len(t, calls[0], 6)
	requirement := strings.TrimPrefix(calls[0][4], "-R=")
	assert.Contains(t, requirement, "anchor apple generic")
	assert.Contains(t, requirement, `identifier "co.autopus.desktop"`)
	assert.Contains(t, requirement, `certificate leaf[subject.OU] = "GP2PFA2PUV"`)
	assert.Contains(t, requirement, "1.2.840.113635.100.6.2.6")
	assert.Contains(t, requirement, "1.2.840.113635.100.6.1.13")
	assert.Equal(t, []string{"--display", "--verbose=4", "--all-architectures",
		"/release/Autopus Desktop.app"}, calls[1])
}

func TestDesktopReleaseIdentity_RejectsWeakMetadataAfterCodesignRunnerSuccess(t *testing.T) {
	t.Parallel()
	variants := map[string]string{
		"self-signed-style": strings.ReplaceAll(desktopCodesignFixtureMetadata(),
			"Developer ID Application: Autopus, Inc. (GP2PFA2PUV)",
			"Self Signed Developer: Attacker (GP2PFA2PUV)"),
		"wrong-intermediate": strings.ReplaceAll(desktopCodesignFixtureMetadata(),
			"Developer ID Certification Authority", "Attacker Certification Authority"),
		"no-runtime": strings.ReplaceAll(desktopCodesignFixtureMetadata(),
			"0x10000(runtime)", "0x0(none)"),
		"wrong-identifier": strings.ReplaceAll(desktopCodesignFixtureMetadata(),
			"co.autopus.desktop", "co.attacker.desktop"),
		"wrong-team": strings.ReplaceAll(desktopCodesignFixtureMetadata(),
			"GP2PFA2PUV", "EVILTEAM00"),
	}
	for name, metadata := range variants {
		metadata := metadata
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner := func(_ context.Context, arguments ...string) (string, error) {
				if arguments[0] == "--verify" {
					return "", nil
				}
				return metadata, nil
			}
			assert.Error(t, verifyDesktopReleaseArtifactIdentityWithRunner(
				context.Background(), "/release/Autopus Desktop.app",
				desktopReleaseBundleIdentifier, desktopReleaseTeamIdentifier, runner,
			))
		})
	}
}

func TestDesktopReleaseIdentity_RejectsCodesignFailureOrOversizedMetadata(t *testing.T) {
	t.Parallel()
	for _, failurePhase := range []string{"verify", "display"} {
		failurePhase := failurePhase
		t.Run(failurePhase, func(t *testing.T) {
			t.Parallel()
			failing := func(_ context.Context, arguments ...string) (string, error) {
				if arguments[0] == "--verify" && failurePhase == "display" {
					return "", nil
				}
				return "", errors.New("codesign rejected artifact")
			}
			assert.Error(t, verifyDesktopReleaseArtifactIdentityWithRunner(
				context.Background(), "/release/Autopus Desktop.app",
				desktopReleaseBundleIdentifier, desktopReleaseTeamIdentifier, failing,
			))
		})
	}
	oversized := func(_ context.Context, arguments ...string) (string, error) {
		if arguments[0] == "--verify" {
			return "", nil
		}
		return strings.Repeat("x", desktopCodesignOutputLimit+1), nil
	}
	assert.Error(t, verifyDesktopReleaseArtifactIdentityWithRunner(
		context.Background(), "/release/Autopus Desktop.app",
		desktopReleaseBundleIdentifier, desktopReleaseTeamIdentifier, oversized,
	))
}

func TestDesktopCodesignOutput_WithOverflow_TruncatesAndFailsClosed(t *testing.T) {
	t.Parallel()
	buffer := &desktopBoundedBuffer{limit: 4}
	written, err := buffer.Write([]byte("abcdef"))
	require.NoError(t, err)
	assert.Equal(t, 6, written)
	assert.Equal(t, "abcd", buffer.String())
	assert.True(t, buffer.overflow)
	written, err = buffer.Write([]byte("next"))
	require.NoError(t, err)
	assert.Equal(t, 4, written)
	assert.Equal(t, "abcd", buffer.String())
}

func TestDesktopReleaseIdentity_RejectsUnsignedBundleBeforeProviderExecution(t *testing.T) {
	t.Parallel()
	artifactPath, callsPath := writeControlledDesktopProvider(t, t.TempDir())
	resolver := &processDesktopProviderResolver{artifactPath: artifactPath}
	client, err := resolver.ResolveLocal(context.Background())
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	assert.Nil(t, client)
	assert.NoFileExists(t, callsPath)
}

func TestDesktopReleaseIdentity_RejectsSymlinkedBundleAndExecutable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realBundle, _ := writeControlledDesktopProvider(t, filepath.Join(root, "real"))
	symlinkBundle := filepath.Join(root, "Linked.app")
	require.NoError(t, os.Symlink(realBundle, symlinkBundle))
	_, err := desktopLocalProviderExecutable(symlinkBundle)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)

	linkedExecutableBundle := filepath.Join(root, "Executable Link.app")
	executableDir := filepath.Join(linkedExecutableBundle, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(executableDir, 0o755))
	require.NoError(t, os.Symlink(
		filepath.Join(realBundle, "Contents", "MacOS", desktopProviderExecutableName),
		filepath.Join(executableDir, desktopProviderExecutableName),
	))
	_, err = desktopLocalProviderExecutable(linkedExecutableBundle)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
}

func TestDesktopReleaseIdentity_VerificationUsesOperationContext(t *testing.T) {
	t.Parallel()
	artifactPath, _ := writeControlledDesktopProvider(t, t.TempDir())
	resolver := &processDesktopProviderResolver{
		artifactPath: artifactPath,
		verifier: func(ctx context.Context, _ string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	client, err := resolver.ResolveLocal(ctx)
	assert.ErrorIs(t, err, errDesktopProviderUnavailable)
	assert.Nil(t, client)
}

func TestDesktopReleaseIdentity_RejectsPostResolutionExecutableSwapBeforeExecution(t *testing.T) {
	if !secureDesktopSpawnSupported() {
		t.Skip("post-resolution execution identity requires secure Darwin cgo spawn")
	}
	t.Parallel()
	for _, replacement := range []string{"regular", "symlink", "in_place", "large"} {
		t.Run(replacement, func(t *testing.T) {
			artifactPath, _ := writeControlledDesktopProvider(t, t.TempDir())
			verificationCalls := 0
			resolver := &processDesktopProviderResolver{
				artifactPath: artifactPath,
				verifier: func(context.Context, string) error {
					verificationCalls++
					return nil
				},
			}
			client, err := resolver.ResolveLocal(context.Background())
			require.NoError(t, err)
			executable := filepath.Join(artifactPath, "Contents", "MacOS", desktopProviderExecutableName)
			verifiedCopy := executable + ".verified"
			sentinel := filepath.Join(t.TempDir(), "executed")
			script := fmt.Sprintf("#!/bin/sh\nprintf x > '%s'\nexit 0\n", sentinel)
			if replacement == "in_place" {
				require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))
			} else {
				require.NoError(t, os.Rename(executable, verifiedCopy))
			}
			if replacement == "symlink" {
				require.NoError(t, os.Symlink(verifiedCopy, executable))
			} else if replacement == "regular" || replacement == "large" {
				require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))
				if replacement == "large" {
					require.NoError(t, os.Truncate(executable, maxDesktopProviderExecutableBytes+1))
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			started := time.Now()
			_, err = client.Handshake(ctx)
			assert.Error(t, err)
			assert.Less(t, time.Since(started), 500*time.Millisecond)
			assert.NoFileExists(t, sentinel)
			assert.Equal(t, 1, verificationCalls)
		})
	}
}

func TestDesktopReleaseIdentity_ConfiguredSignedArtifact(t *testing.T) {
	artifactPath := os.Getenv(desktopSignedArtifactEnvironment)
	if artifactPath == "" {
		t.Skip("release-signed artifact is not configured")
	}
	require.NoError(t, verifyDesktopReleaseArtifact(context.Background(), artifactPath))
	assert.Error(t, verifyDesktopReleaseArtifactIdentity(
		context.Background(), artifactPath, "co.autopus.wrong", desktopReleaseTeamIdentifier,
	))
	assert.Error(t, verifyDesktopReleaseArtifactIdentity(
		context.Background(), artifactPath, desktopReleaseBundleIdentifier, "WRONGTEAM00",
	))
	_, err := desktopLocalProviderExecutable(artifactPath)
	require.NoError(t, err)
}

func desktopCodesignFixtureMetadata() string {
	return strings.Join([]string{
		"Executable=/release/Autopus Desktop.app/Contents/MacOS/autopus-desktop",
		"Identifier=co.autopus.desktop",
		"CodeDirectory v=20500 size=2048 flags=0x10000(runtime) hashes=32+7 location=embedded",
		"Authority=Developer ID Application: Autopus, Inc. (GP2PFA2PUV)",
		"Authority=Developer ID Certification Authority",
		"Authority=Apple Root CA",
		"TeamIdentifier=GP2PFA2PUV",
		"Runtime Version=14.0.0",
	}, "\n")
}
