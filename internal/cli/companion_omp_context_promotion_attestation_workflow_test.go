package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ompContextPromoteWorkflow struct {
	On map[string]struct {
		Inputs map[string]struct {
			Required bool   `yaml:"required"`
			Type     string `yaml:"type"`
		} `yaml:"inputs"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]any    `yaml:"jobs"`
}

func TestOMPContextPromoteWorkflow_DispatchOnlyExactInputsAndLeastPermissions(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	var workflow ompContextPromoteWorkflow
	if err := yaml.Unmarshal([]byte(source), &workflow); err != nil {
		t.Fatalf("parse promotion workflow: %v", err)
	}
	if len(workflow.On) != 1 {
		t.Fatalf("workflow triggers=%v, want workflow_dispatch only", workflow.On)
	}
	dispatch, ok := workflow.On["workflow_dispatch"]
	if !ok {
		t.Fatalf("workflow triggers=%v, want workflow_dispatch", workflow.On)
	}
	wantInputs := []string{
		"candidate_sha256",
		"evidence_commit",
		"source_commit",
		"source_tree",
		"static_policy_b64",
	}
	gotInputs := make([]string, 0, len(dispatch.Inputs))
	for name, input := range dispatch.Inputs {
		gotInputs = append(gotInputs, name)
		if !input.Required || input.Type != "string" {
			t.Fatalf("dispatch input %s=%#v, want required string", name, input)
		}
	}
	sort.Strings(gotInputs)
	if strings.Join(gotInputs, ",") != strings.Join(wantInputs, ",") {
		t.Fatalf("dispatch inputs=%v, want %v", gotInputs, wantInputs)
	}
	wantPermissions := map[string]string{"actions": "write", "contents": "write"}
	if len(workflow.Permissions) != len(wantPermissions) {
		t.Fatalf("workflow permissions=%v", workflow.Permissions)
	}
	for permission, want := range wantPermissions {
		if workflow.Permissions[permission] != want {
			t.Fatalf("workflow permission %s=%q, want %q", permission, workflow.Permissions[permission], want)
		}
	}
	if len(workflow.Jobs) != 1 || workflow.Jobs["promote"] == nil {
		t.Fatalf("workflow jobs=%v, want one promote job", workflow.Jobs)
	}
}

func TestOMPContextPromoteWorkflow_PinsActionsAndExactPromotionCoordinates(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	wantActions := []string{
		"actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10",
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16",
	}
	usesPattern := regexp.MustCompile(`(?m)^\s*uses:\s+([^\s#]+)`)
	matches := usesPattern.FindAllStringSubmatch(source, -1)
	if len(matches) != len(wantActions) {
		t.Fatalf("workflow uses=%v, want exactly %v", matches, wantActions)
	}
	for index, want := range wantActions {
		if matches[index][1] != want {
			t.Fatalf("workflow action %d=%q, want %q", index, matches[index][1], want)
		}
	}
	for _, required := range []string{
		"omp-context-evidence-v0.50.95",
		"omp-context-promotion-report.v1.json",
		"omp-context-promotion-attestation.v2.json",
		"Insajin/autopus-adk",
		`[[ "$EVIDENCE_COMMIT" =~ ^[0-9a-f]{40}$ ]]`,
		`[[ "$SOURCE_COMMIT" =~ ^[0-9a-f]{40}$ ]]`,
		`[[ "$SOURCE_TREE" =~ ^[0-9a-f]{40}$ ]]`,
		`[[ "$CANDIDATE_SHA256" =~ ^[0-9a-f]{64}$ ]]`,
		`[[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$SOURCE_TREE" ]]`,
		`--candidate-revision "$SOURCE_COMMIT"`,
		`--candidate-tree "$SOURCE_TREE"`,
		`--candidate-artifact-sha256 "$CANDIDATE_SHA256"`,
		`--static-policy-b64 "$STATIC_POLICY_B64"`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("workflow is missing exact coordinate contract %q", required)
		}
	}
	releaseBody, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release.yaml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	evidenceFilenamePattern := regexp.MustCompile(`[A-Za-z0-9_.-]*promotion-(report|attestation)[A-Za-z0-9_.-]*\.json`)
	for workflowName, workflowSource := range map[string]string{
		"promotion": source,
		"release":   string(releaseBody),
	} {
		filenameSet := make(map[string]struct{})
		for _, filename := range evidenceFilenamePattern.FindAllString(workflowSource, -1) {
			filenameSet[filename] = struct{}{}
		}
		gotFilenames := make([]string, 0, len(filenameSet))
		for filename := range filenameSet {
			gotFilenames = append(gotFilenames, filename)
		}
		sort.Strings(gotFilenames)
		wantFilenames := []string{
			"omp-context-promotion-attestation.v2.json",
			"omp-context-promotion-report.v1.json",
		}
		if strings.Join(gotFilenames, ",") != strings.Join(wantFilenames, ",") {
			t.Fatalf("%s workflow promotion evidence filenames=%v, want exactly %v",
				workflowName, gotFilenames, wantFilenames)
		}
	}
}

func TestOMPContextPromoteWorkflow_VerifiesOrphanTreeBeforePushAndLeavesVariablesToOperator(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	for _, required := range []string{
		`git fetch --no-tags --depth=1 origin "$EVIDENCE_COMMIT"`,
		`mapfile -d '' -t evidence_entries < <(git ls-tree -z "$EVIDENCE_COMMIT")`,
		`[[ "${#evidence_entries[@]}" -eq 1 ]]`,
		`git cat-file blob "$report_blob" >"$report_path"`,
		`[[ "$published_report_blob" == "$report_blob" ]]`,
		`printf '%s' "$OMP_CONTEXT_EVIDENCE_SIGNING_KEY" |`,
		`env -i PATH="$PATH" HOME="$HOME" TMPDIR="$RUNNER_TEMP"`,
		`companion-manifest omp-context-promotion-attestation`,
		`--mode historical`,
		`--cacheinfo "100644,$published_report_blob,$report_name"`,
		`--cacheinfo "100644,$published_attestation_blob,$attestation_name"`,
		`[[ "${#published_entries[@]}" -eq 2 ]]`,
		`commit-tree "$evidence_tree"`,
		`git rev-list --parents -n 1 "$published_evidence_commit"`,
		`tag -a "$evidence_tag"`,
		`cmp "$report_path" "$verify_dir/$report_name"`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("workflow is missing publication safety contract %q", required)
		}
	}
	if strings.Contains(source, `commit-tree "$evidence_tree" -p`) ||
		strings.Contains(source, `commit-tree -p`) {
		t.Fatal("workflow gives the evidence commit a parent")
	}
	verifyIndex := strings.Index(source, `--mode historical`)
	pushIndex := strings.Index(source, `git push origin "$tag_ref:$tag_ref"`)
	if verifyIndex < 0 || pushIndex <= verifyIndex {
		t.Fatalf("workflow order verify=%d push=%d", verifyIndex, pushIndex)
	}
	if strings.Contains(source, "gh variable set") {
		t.Fatal("promotion workflow writes repository variables instead of leaving publication to the operator")
	}
}

func TestOMPContextPromoteWorkflow_ExposesNoProviderCredentialsOrRawSigningSecret(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	secretPattern := regexp.MustCompile(`\$\{\{\s*secrets\.([A-Z0-9_]+)\s*\}\}`)
	secretMatches := secretPattern.FindAllStringSubmatch(source, -1)
	if len(secretMatches) != 1 || secretMatches[0][1] != "OMP_CONTEXT_EVIDENCE_SIGNING_KEY" {
		t.Fatalf("workflow secret references=%v", secretMatches)
	}
	for _, forbidden := range []string{
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"GOOGLE_API_KEY",
		"AZURE_OPENAI",
		"AWS_ACCESS_KEY",
		"--private-key",
		"--signing-key",
		"set -x",
		`echo "$OMP_CONTEXT_EVIDENCE_SIGNING_KEY"`,
		`printf "$OMP_CONTEXT_EVIDENCE_SIGNING_KEY"`,
		"$GITHUB_ENV",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("workflow contains forbidden credential surface %q", forbidden)
		}
	}
}

func readOMPContextPromoteWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "omp-context-promote.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read promotion workflow: %v", err)
	}
	return string(body)
}
