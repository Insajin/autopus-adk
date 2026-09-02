package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// installedManifestClaims maps every managed path a platform manifest claims to
// the checksum the installer recorded when it wrote that file.
//
// The source-of-truth check above answers "did the generator's input change in
// this commit", which only has an answer inside the ADK repo: it looks for
// staged paths under content/, pkg/adapter/, pkg/content/, pkg/setup/ and
// templates/. A consumer repo has none of those directories, so every
// `auto update` result was unconditionally blocked there -- the requirement was
// not strict, it was unsatisfiable, and the only way to land a regeneration was
// to bypass the hook entirely.
//
// The manifest answers a different and locally decidable question: "is this
// byte-for-byte what the installer wrote?" A hand edit to a generated file
// changes the content without changing the recorded claim, so it still blocks.
// That keeps the guard's purpose while giving consumer repos a reachable path.
func installedManifestClaims(dir string) map[string]string {
	entries, err := os.ReadDir(filepath.Join(dir, ".autopus"))
	if err != nil {
		return nil
	}
	claims := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "-manifest.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, ".autopus", name))
		if err != nil {
			continue
		}
		var manifest adapter.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		for path, file := range manifest.Files {
			if file.Checksum == "" {
				continue
			}
			claims[normalizeGitRel(path)] = file.Checksum
		}
	}
	return claims
}

// stagedMatchesInstalledClaim compares the staged blob -- not the working tree --
// against the manifest claim. Staging is what the commit will contain, and a
// working-tree read would let a staged hand edit pass whenever the file was
// reverted on disk afterwards.
func stagedMatchesInstalledClaim(dir, rel string, claims map[string]string) bool {
	want, ok := claims[normalizeGitRel(rel)]
	if !ok {
		return false
	}
	cmd := exec.Command("git", "show", ":"+rel)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return false
	}
	return adapter.Checksum(buf.String()) == want
}
