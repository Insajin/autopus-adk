package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type verifyProjectSelection struct {
	Filters  []string
	NoFilter bool
}

// runPlaywright executes Playwright while keeping proof policy findings out of process errors.
func runPlaywright(selector string) (output []byte, resultErr error) {
	selection := parseVerifyProjectSelection(selector)
	tempDir, err := os.MkdirTemp("", "autopus-verify-playwright-")
	if err != nil {
		return nil, fmt.Errorf("playwright 임시 디렉터리 생성 실패: %w", err)
	}
	defer func() {
		resultErr = cleanupPlaywrightTempDir(tempDir, resultErr, os.RemoveAll)
	}()
	blobPath, err := filepath.Abs(filepath.Join(tempDir, "report.zip"))
	if err != nil {
		return nil, fmt.Errorf("playwright blob 경로 생성 실패: %w", err)
	}
	reporterPath, proofPath, proofNonce, err := prepareSnapshotProofReporter(tempDir)
	if err != nil {
		return nil, err
	}
	args := []string{
		"playwright", "test",
		"--reporter=json,blob," + reporterPath,
		"--update-snapshots=none",
	}
	for _, project := range selection.Filters {
		args = append(args, "--project="+project)
	}

	cmd := exec.Command("npx", args...)
	cmd.Env = withEnvironmentValue(os.Environ(), "PLAYWRIGHT_BLOB_OUTPUT_FILE", blobPath)
	cmd.Env = withEnvironmentValue(cmd.Env, "AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE", proofPath)
	cmd.Env = withEnvironmentValue(cmd.Env, "AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE", proofNonce)
	stdout := &boundedOutput{limit: maxPlaywrightOutputBytes}
	stderr := &boundedOutput{limit: maxPlaywrightStderrBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	processErr := cmd.Run()
	out, blobLoaded, blobErr := preferredPlaywrightOutput(stdout.Bytes(), blobPath)
	var outputErr error
	if !blobLoaded {
		outputErr = validatePlaywrightJSONOutput(out)
	}

	proof, proofErr := readSnapshotComparisonProof(proofPath, proofNonce)
	if proofErr != nil {
		proof = missingSnapshotProof(selection, classifySnapshotProofError(proofErr))
	}
	proof.RequiredProjects = requiredSnapshotProjects(proof, selection)
	proof.Diagnostic = assessSnapshotProof(proof, selection)
	annotated, annotationErr := annotatePlaywrightProof(out, blobLoaded, proof)
	var finalBlobErr error
	if annotationErr == nil {
		out = annotated
		if blobLoaded {
			if err := validatePlaywrightBlob(out); err != nil {
				finalBlobErr = fmt.Errorf("playwright annotated blob 검증 실패: %w", err)
			}
		}
	} else {
		proof.Diagnostic = strings.Join([]string{proof.Diagnostic, "snapshot proof evidence packaging was unavailable"}, "; ")
		out, _ = minimalSnapshotProofOutput(proof)
	}
	if blobLoaded && (blobErr != nil || finalBlobErr != nil) {
		fallback, fallbackErr := minimalSnapshotProofOutput(proof)
		if fallbackErr != nil {
			annotationErr = errors.Join(annotationErr, fallbackErr)
		} else {
			out = fallback
		}
	}

	hardErr := errors.Join(playwrightProcessError(processErr, stderr.Bytes()), blobErr, finalBlobErr, outputErr)
	if annotationErr != nil {
		hardErr = errors.Join(hardErr, fmt.Errorf("playwright blob proof 기록 실패: %w", annotationErr))
	}
	if stdout.overflow {
		hardErr = errors.Join(hardErr, fmt.Errorf("playwright stdout이 %d바이트 제한을 초과했습니다", maxPlaywrightOutputBytes))
	}
	if stderr.overflow {
		hardErr = errors.Join(hardErr, fmt.Errorf("playwright stderr가 %d바이트 제한을 초과했습니다", maxPlaywrightStderrBytes))
	}
	return out, hardErr
}

func parseVerifyProjectSelection(selector string) verifyProjectSelection {
	selector = strings.TrimSpace(selector)
	if selector == "" || selector == "desktop" {
		return verifyProjectSelection{NoFilter: true}
	}
	seen := map[string]struct{}{}
	selection := verifyProjectSelection{}
	for _, value := range strings.Split(selector, ",") {
		project := strings.TrimSpace(value)
		if project == "" {
			continue
		}
		if _, duplicate := seen[project]; duplicate {
			continue
		}
		seen[project] = struct{}{}
		selection.Filters = append(selection.Filters, project)
	}
	selection.NoFilter = len(selection.Filters) == 0
	return selection
}

func preferredPlaywrightOutput(stdout []byte, blobPath string) ([]byte, bool, error) {
	out := append([]byte(nil), stdout...)
	info, err := os.Lstat(blobPath)
	if os.IsNotExist(err) {
		return out, false, nil
	}
	if err != nil {
		return out, false, fmt.Errorf("playwright blob 상태 확인 실패: %w", err)
	}
	if !info.Mode().IsRegular() {
		return out, false, fmt.Errorf("playwright blob이 regular file이 아닙니다")
	}
	if info.Size() == 0 {
		return out, false, nil
	}
	if info.Size() > maxBlobArchiveBytes {
		return out, false, fmt.Errorf("playwright blob이 %d바이트 제한을 초과했습니다", maxBlobArchiveBytes)
	}
	blob, err := os.ReadFile(blobPath)
	if err != nil {
		return out, false, fmt.Errorf("playwright blob 읽기 실패: %w", err)
	}
	if err := validatePlaywrightBlob(blob); err != nil {
		return blob, true, fmt.Errorf("playwright blob 검증 실패: %w", err)
	}
	return blob, true, nil
}

func annotatePlaywrightProof(output []byte, blob bool, proof snapshotComparisonProof) ([]byte, error) {
	if blob {
		return appendSnapshotProofToBlob(output, proof)
	}
	if json.Valid(output) {
		return annotateSnapshotProofJSON(output, proof)
	}
	return minimalSnapshotProofOutput(proof)
}

func validatePlaywrightJSONOutput(output []byte) error {
	if len(bytes.TrimSpace(output)) == 0 {
		return fmt.Errorf("playwright JSON report가 누락되었습니다")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(output, &object); err != nil {
		return fmt.Errorf("playwright JSON report가 올바르지 않습니다: %w", err)
	}
	if object == nil {
		return fmt.Errorf("playwright JSON report가 객체가 아닙니다")
	}
	return nil
}

func minimalSnapshotProofOutput(proof snapshotComparisonProof) ([]byte, error) {
	proofRaw, err := json.Marshal(proof)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{
		"suites":                          json.RawMessage("[]"),
		"_autopusSnapshotComparisonProof": proofRaw,
	})
}

func missingSnapshotProof(selection verifyProjectSelection, diagnostic string) snapshotComparisonProof {
	names := append([]string(nil), selection.Filters...)
	if len(names) == 0 {
		names = []string{"unresolved-project"}
	}
	proof := snapshotComparisonProof{
		Version: 2, Nonce: "synthetic", PlaywrightVersion: "unavailable",
		UpdateSnapshots: "missing", Diagnostic: diagnostic,
	}
	for _, name := range names {
		proof.Projects = append(proof.Projects, snapshotProjectProof{Name: name, State: "unproven", Source: "missing"})
	}
	return proof
}

func classifySnapshotProofError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "누락"):
		return "snapshot comparison proof is missing"
	case strings.Contains(message, "nonce"):
		return "snapshot comparison proof authenticity could not be established"
	default:
		return "snapshot comparison proof is invalid"
	}
}

func assessSnapshotProof(proof snapshotComparisonProof, selection verifyProjectSelection) string {
	required := proof.RequiredProjects
	if len(required) == 0 {
		required = requiredSnapshotProjects(proof, selection)
	}
	projects := make(map[string]snapshotProjectProof, len(proof.Projects))
	for _, project := range proof.Projects {
		projects[project.Name] = project
	}
	var issues []string
	if proof.UpdateSnapshots != "none" {
		issues = append(issues, "updateSnapshots is not proven to be none")
	}
	for _, name := range required {
		project, ok := projects[name]
		if !ok {
			issues = append(issues, fmt.Sprintf("project %q has no snapshot proof", name))
			continue
		}
		if project.State != "enabled" {
			issues = append(issues, fmt.Sprintf("project %q snapshot comparison is %s", name, project.State))
		}
	}
	if len(required) == 0 {
		issues = append(issues, "no required Playwright project was proven")
	}
	if proof.Diagnostic != "" {
		issues = append([]string{proof.Diagnostic}, issues...)
	}
	return strings.Join(issues, "; ")
}

func requiredSnapshotProjects(proof snapshotComparisonProof, selection verifyProjectSelection) []string {
	if !selection.NoFilter {
		return append([]string(nil), selection.Filters...)
	}
	var required []string
	for _, project := range proof.Projects {
		if !project.SupportOnly {
			required = append(required, project.Name)
		}
	}
	sort.Strings(required)
	return required
}

func playwrightProcessError(processErr error, stderr []byte) error {
	if processErr == nil {
		return nil
	}
	diagnostic := sanitizePlaywrightStderr(stderr)
	safeProcessErr := wrapSanitizedPlaywrightError(processErr)
	if diagnostic == "" {
		return fmt.Errorf("playwright 실행 오류 (종료 코드 포함): %w", safeProcessErr)
	}
	return fmt.Errorf("playwright 실행 오류 (종료 코드 포함): %w\nstderr: %s", safeProcessErr, diagnostic)
}

func withEnvironmentValue(environment []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}
