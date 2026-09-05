package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// dirtyFile is one merged porcelain status entry for a worktree path.
type dirtyFile struct {
	Rel      string
	Staged   bool
	Unstaged bool
	Missing  bool
}

// gitPorcelainSnapshot is one read-only capture of a worktree's porcelain
// status: the canonical bytes git emitted plus the parsed, path-sorted entries.
type gitPorcelainSnapshot struct {
	Raw   []byte
	Files []dirtyFile
}

// capturePorcelainSnapshot runs the read-only status inventory for dir.
// repoLabel only decorates diagnostics and is sanitized before display.
func capturePorcelainSnapshot(repoLabel, dir string) (gitPorcelainSnapshot, error) {
	raw, err := runSyncGit(repoLabel, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return gitPorcelainSnapshot{}, err
	}
	files, err := parsePorcelainXY(raw)
	if err != nil {
		return gitPorcelainSnapshot{}, fmt.Errorf("malformed git status for repo %s", diagnosticRepoLabel(repoLabel))
	}
	return gitPorcelainSnapshot{Raw: raw, Files: files}, nil
}

func runSyncGit(repoLabel, dir string, args ...string) ([]byte, error) {
	gitArgs := append([]string{"--no-optional-locks"}, args...)
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Git stderr is intentionally discarded: it can contain absolute paths,
	// credentials embedded in remotes, or attacker-controlled local text.
	if err := cmd.Run(); err != nil {
		op := "command"
		if len(args) > 0 {
			op = args[0]
		}
		return nil, fmt.Errorf("read-only git %s failed for repo %s", op, diagnosticRepoLabel(repoLabel))
	}
	return stdout.Bytes(), nil
}

func parsePorcelainXY(raw []byte) ([]dirtyFile, error) {
	var out []dirtyFile
	for offset := 0; offset < len(raw); {
		record, next, ok := nextNULRecord(raw, offset)
		if !ok || len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("invalid porcelain record")
		}
		x, y := record[0], record[1]
		if !validPorcelainCode(x) || !validPorcelainCode(y) {
			return nil, fmt.Errorf("invalid porcelain status")
		}
		out = appendDirtyPath(out, string(record[3:]), x, y, x == 'D' || y == 'D')
		offset = next
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			source, afterSource, sourceOK := nextNULRecord(raw, offset)
			if !sourceOK || len(source) == 0 {
				return nil, fmt.Errorf("rename source missing")
			}
			if x == 'R' || y == 'R' {
				out = appendRenameSource(out, string(source), x, y)
			}
			offset = afterSource
		}
	}
	return mergeDirtyFiles(out), nil
}

func nextNULRecord(raw []byte, offset int) ([]byte, int, bool) {
	if offset >= len(raw) {
		return nil, offset, false
	}
	idx := bytes.IndexByte(raw[offset:], 0)
	if idx < 0 {
		return nil, offset, false
	}
	end := offset + idx
	return raw[offset:end], end + 1, true
}

func validPorcelainCode(code byte) bool {
	return strings.ContainsRune(" MTADRCU?!", rune(code))
}

func appendDirtyPath(files []dirtyFile, rel string, x, y byte, missing bool) []dirtyFile {
	if rel == "" {
		return files
	}
	untracked := x == '?' && y == '?'
	return append(files, dirtyFile{
		Rel:      normalizeGitRel(rel),
		Staged:   x != ' ' && x != '?',
		Unstaged: y != ' ' || untracked,
		Missing:  missing,
	})
}

func appendRenameSource(files []dirtyFile, rel string, x, y byte) []dirtyFile {
	if rel == "" {
		return files
	}
	return append(files, dirtyFile{
		Rel:      normalizeGitRel(rel),
		Staged:   x == 'R',
		Unstaged: y == 'R',
		Missing:  true,
	})
}

func mergeDirtyFiles(files []dirtyFile) []dirtyFile {
	merged := map[string]dirtyFile{}
	for _, file := range files {
		if file.Rel == "" {
			continue
		}
		current, seen := merged[file.Rel]
		current.Rel = file.Rel
		current.Staged = current.Staged || file.Staged
		current.Unstaged = current.Unstaged || file.Unstaged
		if seen {
			current.Missing = current.Missing && file.Missing
		} else {
			current.Missing = file.Missing
		}
		merged[file.Rel] = current
	}
	out := make([]dirtyFile, 0, len(merged))
	for _, file := range merged {
		out = append(out, file)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out
}
