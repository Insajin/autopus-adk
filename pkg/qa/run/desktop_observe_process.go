package run

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const (
	desktopSignedArtifactEnvironment = "AUTOPUS_RELEASE_SIGNED_ARTIFACT_PATH"
	desktopProviderExecutableName    = "autopus-desktop"
	desktopProvenanceMarker          = "autopus.desktop-observe.candidate.release.v2"
	desktopSupervisorRecipe          = "autopus.desktop-observe.supervisor-recipe.v2"
)

var errDesktopProviderUnavailable = errors.New("selected desktop observation provider is unavailable")

type processDesktopProviderResolver struct {
	artifactPath string
	verifier     desktopArtifactVerifier
}

func newProductionDesktopObservationRunner(opts Options) *desktopObservationRunner {
	return newDesktopObservationRunnerWithResolver(&processDesktopProviderResolver{
		artifactPath: opts.desktopArtifactPath,
		verifier:     opts.desktopArtifactVerifier,
	})
}

func (resolver *processDesktopProviderResolver) ResolveLocal(
	ctx context.Context,
) (desktopProviderClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	artifactPath := resolver.artifactPath
	if artifactPath == "" {
		artifactPath = os.Getenv(desktopSignedArtifactEnvironment)
	}
	executable, err := desktopLocalProviderExecutable(artifactPath)
	if err != nil {
		return nil, errDesktopProviderUnavailable
	}
	resolvedArtifact, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		return nil, errDesktopProviderUnavailable
	}
	resolvedExecutable, err := filepath.EvalSymlinks(executable)
	if err != nil || resolvedExecutable != filepath.Join(
		resolvedArtifact, "Contents", "MacOS", desktopProviderExecutableName,
	) {
		return nil, errDesktopProviderUnavailable
	}
	artifactPath, executable = resolvedArtifact, resolvedExecutable
	verifier := resolver.verifier
	if verifier == nil {
		verifier = verifyDesktopReleaseArtifact
	}
	if err := verifier(ctx, artifactPath); err != nil {
		return nil, errDesktopProviderUnavailable
	}
	executableInfo, executableDigest, codeIdentity, err := snapshotDesktopExecutable(ctx, executable, -1)
	if err != nil {
		return nil, errDesktopProviderUnavailable
	}
	fileIdentity, err := desktopExecutableFileIdentity(executableInfo)
	if err != nil || !secureDesktopSpawnSupported() {
		return nil, errDesktopProviderUnavailable
	}
	transport := &processDesktopEnvelopeTransport{
		command:          executable,
		artifactPath:     artifactPath,
		executableInfo:   executableInfo,
		executableDigest: executableDigest,
		codeIdentity:     codeIdentity,
		fileIdentity:     fileIdentity,
		arguments: []string{
			"--desktop-observe-spike-provider", "--protocol-version", "2",
		},
		environment: desktopProviderEnvironment(artifactPath),
	}
	client := newEnvelopeDesktopClient(
		expectedDesktopIdentity(desktopobserve.RuntimeProviderLocal),
		transport,
		desktopobserve.DecodeExchange,
	)
	client.handshakeViaCapabilities = true
	return client, nil
}

func (*processDesktopProviderResolver) LookPath(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", errDesktopProviderUnavailable
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return path, nil
}

func (*processDesktopProviderResolver) ResolveOrca(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || !filepath.IsAbs(resolved) || !desktopExecutableFile(resolved) {
		return "", errDesktopProviderUnavailable
	}
	return resolved, nil
}

func (*processDesktopProviderResolver) NewOrcaClient(
	ctx context.Context,
	path string,
) (desktopProviderClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return newOrcaDesktopClient(path)
}

func desktopLocalProviderExecutable(artifactPath string) (string, error) {
	if artifactPath == "" || !filepath.IsAbs(artifactPath) || filepath.Ext(artifactPath) != ".app" {
		return "", errDesktopProviderUnavailable
	}
	for _, path := range []string{
		artifactPath,
		filepath.Join(artifactPath, "Contents"),
		filepath.Join(artifactPath, "Contents", "MacOS"),
	} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", errDesktopProviderUnavailable
		}
	}
	executable := filepath.Join(artifactPath, "Contents", "MacOS", desktopProviderExecutableName)
	info, err := os.Lstat(executable)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", errDesktopProviderUnavailable
	}
	return executable, nil
}

func desktopExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

type processDesktopEnvelopeTransport struct {
	command          string
	artifactPath     string
	arguments        []string
	environment      []string
	executableInfo   os.FileInfo
	executableDigest desktopExecutableDigest
	codeIdentity     desktopCodeIdentity
	fileIdentity     desktopFileIdentity
}

func (transport *processDesktopEnvelopeTransport) RoundTrip(
	ctx context.Context,
	requestRaw []byte,
) ([]byte, error) {
	if transport == nil || transport.verifyIdentity(ctx) != nil {
		return nil, errDesktopProviderUnavailable
	}
	privateRequestRaw, binding, err := encodeDesktopV2Request(requestRaw)
	if err != nil {
		return nil, err
	}
	result, runErr := runSecureDesktopCommand(ctx, secureDesktopSpawnSpec{
		command:      transport.command,
		arguments:    append([]string(nil), transport.arguments...),
		environment:  append([]string(nil), transport.environment...),
		codeIdentity: transport.codeIdentity,
		fileIdentity: transport.fileIdentity,
	}, privateRequestRaw)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if runErr != nil {
		return nil, runErr
	}
	if len(result.stderr) != 0 {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	privateResultRaw := append([]byte(nil), result.stdout...)
	if len(privateResultRaw) == 0 {
		return nil, errDesktopProviderUnavailable
	}
	resultRaw, privateStatus, decodeErr := translateDesktopV2Result(binding, privateResultRaw)
	if decodeErr != nil {
		return nil, decodeErr
	}
	if privateStatus == desktopV2StatusOK {
		if result.exitCode != 0 {
			return nil, desktopobserve.ErrMalformedEnvelope
		}
	} else if result.exitCode != 1 {
		return nil, desktopobserve.ErrMalformedEnvelope
	}
	return resultRaw, nil
}

func desktopProviderEnvironment(artifactPath string) []string {
	values := map[string]string{
		"AUTOPUS_Q12_PROVENANCE_MARKER":    desktopProvenanceMarker,
		"AUTOPUS_Q12_SIGNED_ARTIFACT_PATH": artifactPath,
		"AUTOPUS_Q12_SPIKE_CANDIDATE_ID":   "rust-go",
		"AUTOPUS_Q12_SUPERVISOR_RECIPE":    desktopSupervisorRecipe,
	}
	for _, key := range []string{"HOME", "LANG", "LC_ALL", "LOGNAME", "PATH", "SHELL", "TMPDIR", "USER"} {
		if value, ok := os.LookupEnv(key); ok {
			values[key] = value
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	environment := make([]string, 0, len(keys))
	for _, key := range keys {
		environment = append(environment, key+"="+values[key])
	}
	return environment
}
