package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maximumHarnessFileBytes = 1024 * 1024

var (
	harnessFiles = []string{"AGENTS.md", "autopus.yaml"}
	contextFiles = []string{
		".autopus/project/workspace.md",
		".autopus/context/constraints.yaml",
	}
)

func Doctor(opts DoctorOptions) (DoctorReceipt, error) {
	empty := DoctorReceipt{}
	if ValidateOpaqueRepoScopeRef(opts.RepoScopeRef) != nil || ValidatePhase(opts.Phase) != nil {
		return empty, convergenceError(ReasonScopeInvalid)
	}
	root, err := scopedWorktreeRoot(opts.WorkingDirectory)
	if err != nil {
		return empty, convergenceError(ReasonScopeInvalid)
	}
	harnessDigest, err := digestHarnessSet(root, harnessFiles)
	if err != nil {
		return empty, convergenceError(ReasonHarnessInvalid)
	}
	contextDigest, err := digestHarnessSet(root, contextFiles)
	if err != nil {
		return empty, convergenceError(ReasonHarnessInvalid)
	}
	return DoctorReceipt{
		SchemaVersion:  DeliveryDoctorSchemaV1,
		Status:         "ready",
		RepoScopeRef:   opts.RepoScopeRef,
		Phase:          opts.Phase,
		ScopedWorktree: true,
		HarnessDigest:  harnessDigest,
		ContextDigest:  contextDigest,
	}, nil
}

func ValidateOpaqueRepoScopeRef(value string) error {
	const prefix = "repo-"
	if !strings.HasPrefix(value, prefix) || len(value) < len(prefix)+2 || len(value) > 69 {
		return convergenceError(ReasonScopeInvalid)
	}
	for index, character := range []byte(value[len(prefix):]) {
		valid := character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			index > 0 && (character == '-' || character == '_' || character == '.')
		if !valid {
			return convergenceError(ReasonScopeInvalid)
		}
	}
	return nil
}

func scopedWorktreeRoot(directory string) (string, error) {
	if directory == "" {
		var err error
		directory, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	root, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", convergenceError(ReasonScopeInvalid)
	}
	info, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil || !info.Mode().IsRegular() {
		return "", convergenceError(ReasonScopeInvalid)
	}
	inside, err := runGit(root, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return "", convergenceError(ReasonScopeInvalid)
	}
	top, err := runGit(root, "rev-parse", "--show-toplevel")
	if err != nil || !sameCanonicalPath(root, strings.TrimSpace(top)) {
		return "", convergenceError(ReasonScopeInvalid)
	}
	if _, err := runGit(root, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return "", convergenceError(ReasonScopeInvalid)
	}
	gitDirectory, err := runGit(root, "rev-parse", "--git-dir")
	if err != nil {
		return "", convergenceError(ReasonScopeInvalid)
	}
	commonDirectory, err := runGit(root, "rev-parse", "--git-common-dir")
	if err != nil || sameCanonicalPath(
		resolveGitPath(root, gitDirectory), resolveGitPath(root, commonDirectory),
	) {
		return "", convergenceError(ReasonScopeInvalid)
	}
	return root, nil
}

func runGit(directory string, arguments ...string) (string, error) {
	args := append([]string{"-C", directory}, arguments...)
	command := exec.Command("git", args...)
	command.Env = safeGitEnvironment()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return "", convergenceError(ReasonScopeInvalid)
	}
	return output.String(), nil
}

func safeGitEnvironment() []string {
	unsafe := []string{
		"GIT_DIR=", "GIT_WORK_TREE=", "GIT_COMMON_DIR=", "GIT_INDEX_FILE=",
		"GIT_OBJECT_DIRECTORY=", "GIT_ALTERNATE_OBJECT_DIRECTORIES=",
	}
	result := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		blocked := false
		for _, prefix := range unsafe {
			if strings.HasPrefix(entry, prefix) {
				blocked = true
				break
			}
		}
		if !blocked {
			result = append(result, entry)
		}
	}
	return result
}

func resolveGitPath(root, value string) string {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Join(root, value)
}

func sameCanonicalPath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return filepath.Clean(leftResolved) == filepath.Clean(rightResolved)
}

func digestHarnessSet(root string, names []string) (string, error) {
	hasher := sha256.New()
	for _, name := range names {
		content, err := readScopedRegularBounded(root, name)
		if err != nil || len(bytes.TrimSpace(content)) == 0 || !validHarnessContent(name, content) {
			return "", convergenceError(ReasonHarnessInvalid)
		}
		writeDigestFrame(hasher, []byte(name))
		writeDigestFrame(hasher, content)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func readScopedRegularBounded(root, relative string) ([]byte, error) {
	parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	current := root
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, convergenceError(ReasonHarnessInvalid)
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, convergenceError(ReasonHarnessInvalid)
		}
		if index+1 < len(parts) && !info.IsDir() {
			return nil, convergenceError(ReasonHarnessInvalid)
		}
	}
	name := current
	before, err := os.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > maximumHarnessFileBytes {
		return nil, convergenceError(ReasonHarnessInvalid)
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, convergenceError(ReasonHarnessInvalid)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, convergenceError(ReasonHarnessInvalid)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumHarnessFileBytes+1))
	if err != nil || len(content) > maximumHarnessFileBytes {
		return nil, convergenceError(ReasonHarnessInvalid)
	}
	return content, nil
}

func validHarnessContent(name string, content []byte) bool {
	if !strings.HasSuffix(name, ".yaml") {
		return true
	}
	var document yaml.Node
	if yaml.Unmarshal(content, &document) != nil || len(document.Content) != 1 {
		return false
	}
	return document.Content[0].Kind == yaml.MappingNode && validYAMLNode(document.Content[0])
}

func validYAMLNode(node *yaml.Node) bool {
	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content)%2 != 0 {
			return false
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Kind != yaml.ScalarNode {
				return false
			}
			if _, duplicate := seen[key.Value]; duplicate {
				return false
			}
			seen[key.Value] = struct{}{}
			if !validYAMLNode(node.Content[index+1]) {
				return false
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if !validYAMLNode(child) {
				return false
			}
		}
	case yaml.AliasNode:
		return false
	}
	return true
}

func writeDigestFrame(writer io.Writer, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}
