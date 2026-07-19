package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const (
	desktopReleaseBundleIdentifier = "co.autopus.desktop"
	desktopReleaseTeamIdentifier   = "GP2PFA2PUV"
	desktopCodesignOutputLimit     = 65_536
)

var desktopCodeDirectoryRuntime = regexp.MustCompile(
	`flags=0x[0-9a-f]+\([^)]*\bruntime\b[^)]*\)`,
)

type desktopArtifactVerifier func(context.Context, string) error

type desktopCodesignRunner func(context.Context, ...string) (string, error)

func verifyDesktopReleaseArtifact(ctx context.Context, artifactPath string) error {
	return verifyDesktopReleaseArtifactIdentity(
		ctx, artifactPath, desktopReleaseBundleIdentifier, desktopReleaseTeamIdentifier,
	)
}

func verifyDesktopReleaseArtifactIdentity(
	ctx context.Context,
	artifactPath string,
	bundleIdentifier string,
	teamIdentifier string,
) error {
	return verifyDesktopReleaseArtifactIdentityWithRunner(
		ctx, artifactPath, bundleIdentifier, teamIdentifier, runDesktopCodesign,
	)
}

func verifyDesktopReleaseArtifactIdentityWithRunner(
	ctx context.Context,
	artifactPath string,
	bundleIdentifier string,
	teamIdentifier string,
	runner desktopCodesignRunner,
) error {
	requirement := desktopReleaseCodeRequirement(bundleIdentifier, teamIdentifier)
	if _, err := runner(ctx,
		"--verify", "--deep", "--strict", "--all-architectures",
		"-R="+requirement, artifactPath,
	); err != nil {
		return fmt.Errorf("verify desktop release signature: %w", err)
	}
	metadata, err := runner(ctx,
		"--display", "--verbose=4", "--all-architectures", artifactPath,
	)
	if err != nil {
		return fmt.Errorf("inspect desktop release signature: %w", err)
	}
	if err := verifyDesktopCodesignMetadata(metadata, bundleIdentifier, teamIdentifier); err != nil {
		return fmt.Errorf("verify desktop release metadata: %w", err)
	}
	return nil
}

func desktopReleaseCodeRequirement(bundleIdentifier string, teamIdentifier string) string {
	return `anchor apple generic and identifier "` + bundleIdentifier +
		`" and certificate leaf[subject.OU] = "` + teamIdentifier +
		`" and certificate 1[field.1.2.840.113635.100.6.2.6] exists` +
		` and certificate leaf[field.1.2.840.113635.100.6.1.13] exists`
}

func runDesktopCodesign(ctx context.Context, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, "/usr/bin/codesign", arguments...)
	output := &desktopBoundedBuffer{limit: desktopCodesignOutputLimit}
	command.Stdout = output
	command.Stderr = output
	if err := command.Run(); err != nil {
		return "", err
	}
	if output.overflow {
		return "", errors.New("codesign output exceeds bound")
	}
	return output.String(), nil
}

type desktopBoundedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (buffer *desktopBoundedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := buffer.limit - buffer.Len()
	if remaining <= 0 {
		buffer.overflow = true
		return written, nil
	}
	if len(data) > remaining {
		buffer.overflow = true
		data = data[:remaining]
	}
	_, _ = buffer.Buffer.Write(data)
	return written, nil
}

func verifyDesktopCodesignMetadata(metadata string, bundleIdentifier string, teamIdentifier string) error {
	if len(metadata) == 0 || len(metadata) > desktopCodesignOutputLimit || strings.ContainsRune(metadata, '\x00') {
		return errors.New("invalid codesign metadata")
	}
	lines := strings.Split(strings.ReplaceAll(metadata, "\r\n", "\n"), "\n")
	identifiers := desktopMetadataValues(lines, "Identifier=")
	teams := desktopMetadataValues(lines, "TeamIdentifier=")
	authorities := desktopMetadataValues(lines, "Authority=")
	codeDirectories := desktopMetadataLines(lines, "CodeDirectory ")
	if !desktopAllExact(identifiers, bundleIdentifier) || !desktopAllExact(teams, teamIdentifier) {
		return errors.New("codesign identifier or team mismatch")
	}
	for _, line := range codeDirectories {
		if !desktopCodeDirectoryRuntime.MatchString(line) {
			return errors.New("hardened runtime is required")
		}
	}
	if len(codeDirectories) == 0 || !desktopAuthorityChainValid(authorities, teamIdentifier) {
		return errors.New("Developer ID authority metadata mismatch")
	}
	return nil
}

func desktopMetadataValues(lines []string, prefix string) []string {
	values := make([]string, 0)
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			values = append(values, strings.TrimPrefix(line, prefix))
		}
	}
	return values
}

func desktopMetadataLines(lines []string, prefix string) []string {
	values := make([]string, 0)
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			values = append(values, line)
		}
	}
	return values
}

func desktopAllExact(values []string, expected string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value != expected {
			return false
		}
	}
	return true
}

func desktopAuthorityChainValid(authorities []string, teamIdentifier string) bool {
	leafCount := 0
	intermediate := false
	root := false
	for _, authority := range authorities {
		switch {
		case strings.HasPrefix(authority, "Developer ID Application:"):
			if !strings.HasSuffix(authority, "("+teamIdentifier+")") {
				return false
			}
			leafCount++
		case authority == "Developer ID Certification Authority":
			intermediate = true
		case authority == "Apple Root CA":
			root = true
		}
	}
	return leafCount > 0 && intermediate && root
}
