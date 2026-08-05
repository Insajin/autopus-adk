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
		"dispatch_nonce",
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
	wantPermissions := map[string]string{"contents": "read"}
	if len(workflow.Permissions) != len(wantPermissions) {
		t.Fatalf("workflow permissions=%v", workflow.Permissions)
	}
	for permission, want := range wantPermissions {
		if workflow.Permissions[permission] != want {
			t.Fatalf("workflow permission %s=%q, want %q", permission, workflow.Permissions[permission], want)
		}
	}
	wantJobs := []string{"prepare", "publish", "sign", "verify"}
	gotJobs := make([]string, 0, len(workflow.Jobs))
	for name := range workflow.Jobs {
		gotJobs = append(gotJobs, name)
	}
	sort.Strings(gotJobs)
	if strings.Join(gotJobs, ",") != strings.Join(wantJobs, ",") {
		t.Fatalf("workflow jobs=%v, want %v", gotJobs, wantJobs)
	}
}

func TestOMPContextPromoteWorkflow_PinsActionsAndExactPromotionCoordinates(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	wantActionCounts := map[string]int{
		"actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10":          2,
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16":          1,
		"actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02":   3,
		"actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093": 3,
	}
	usesPattern := regexp.MustCompile(`(?m)^\s*uses:\s+([^\s#]+)`)
	matches := usesPattern.FindAllStringSubmatch(source, -1)
	gotActionCounts := make(map[string]int)
	for _, match := range matches {
		gotActionCounts[match[1]]++
	}
	if len(gotActionCounts) != len(wantActionCounts) {
		t.Fatalf("workflow actions=%v, want %v", gotActionCounts, wantActionCounts)
	}
	for action, want := range wantActionCounts {
		if gotActionCounts[action] != want {
			t.Fatalf("workflow action %s count=%d, want %d", action, gotActionCounts[action], want)
		}
	}
	for _, required := range []string{
		"omp-context-evidence-v0.50.96",
		"omp-context-promotion-report.v1.json",
		"omp-context-promotion-attestation.v2.json",
		"Insajin/autopus-adk",
		`[[ "$EVIDENCE_COMMIT" =~ ^[0-9a-f]{40}$ ]]`,
		`[[ "$SOURCE_COMMIT" =~ ^[0-9a-f]{40}$ && "$SOURCE_TREE" =~ ^[0-9a-f]{40}$ ]]`,
		`[[ "$CANDIDATE_SHA256" =~ ^[0-9a-f]{64}$ ]]`,
		`[[ "$(git rev-parse --verify 'HEAD^{tree}')" == "$SOURCE_TREE" ]]`,
		`--candidate-revision "$SOURCE_COMMIT"`,
		`--candidate-tree "$SOURCE_TREE"`,
		`--candidate-artifact-sha256 "$CANDIDATE_SHA256"`,
		`--static-policy-b64 "$STATIC_POLICY_B64"`,
		`[[ "$GITHUB_ACTOR_ID" == '204883817' ]]`,
		`[[ "$GITHUB_REF" == 'refs/heads/main' && "$GITHUB_SHA" == "$SOURCE_COMMIT" ]]`,
		`refs/heads/omp-context-evidence-v0.50.96-source`,
		`"$EVIDENCE_COMMIT"$'\t'"$lock_ref"`,
		`persist-credentials: false`,
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

func TestOMPContextPromoteWorkflow_SeparatesSigningVerificationAndWriteAuthority(t *testing.T) {
	source := readOMPContextPromoteWorkflow(t)
	for _, required := range []string{
		`git fetch --no-tags --depth=1 "$repository_url"`,
		`mapfile -d '' -t entries < <(git ls-tree -z "$EVIDENCE_COMMIT")`,
		`[[ "${#entries[@]}" -eq 1 ]]`,
		`git cat-file blob "$report_blob" >"$report_path"`,
		`2ZO4NEHN+2yUw3huo8ZIXp/ITGd6WMN+EyiQVc9a3y8=`,
		`crypto.sign(null, message, privateKey)`,
		`--mode historical`,
		`--cacheinfo "100644,$report_blob,omp-context-promotion-report.v1.json"`,
		`--cacheinfo "100644,$attestation_blob,omp-context-promotion-attestation.v2.json"`,
		`commit-tree "$evidence_tree"`,
		`git rev-list --parents -n 1 "$evidence_commit"`,
		`tag -a omp-context-evidence-v0.50.96`,
		`git push --atomic --force-with-lease="${lock_ref}:${EVIDENCE_COMMIT}"`,
		`"$tag_ref:$tag_ref" "$EVIDENCE_COMMIT:$lock_ref"`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("workflow is missing publication safety contract %q", required)
		}
	}
	if strings.Contains(source, `commit-tree "$evidence_tree" -p`) ||
		strings.Contains(source, `commit-tree -p`) {
		t.Fatal("workflow gives the evidence commit a parent")
	}
	signStart := strings.Index(source, "\n  sign:\n")
	verifyStart := strings.Index(source, "\n  verify:\n")
	publishStart := strings.Index(source, "\n  publish:\n")
	if signStart < 0 || verifyStart <= signStart || publishStart <= verifyStart {
		t.Fatalf("workflow trust-boundary jobs are missing: sign=%d verify=%d publish=%d",
			signStart, verifyStart, publishStart)
	}
	signJob := source[signStart:verifyStart]
	verifyJob := source[verifyStart:publishStart]
	publishJob := source[publishStart:]
	for _, forbidden := range []string{"go build", "actions/checkout@", "PROMOTION_PUSH_TOKEN", "contents: write"} {
		if strings.Contains(signJob, forbidden) {
			t.Fatalf("signing job contains source execution or write authority %q", forbidden)
		}
	}
	if !strings.Contains(signJob, "OMP_CONTEXT_EVIDENCE_SIGNING_KEY") {
		t.Fatal("signing job does not own the exact signing secret")
	}
	for _, forbidden := range []string{"OMP_CONTEXT_EVIDENCE_SIGNING_KEY", "PROMOTION_PUSH_TOKEN", "contents: write"} {
		if strings.Contains(verifyJob, forbidden) {
			t.Fatalf("verification job contains credential authority %q", forbidden)
		}
	}
	if !strings.Contains(verifyJob, "go build") || !strings.Contains(verifyJob, "--mode historical") {
		t.Fatal("verification job does not execute the source verifier without credentials")
	}
	for _, forbidden := range []string{"OMP_CONTEXT_EVIDENCE_SIGNING_KEY", "go build", "actions/checkout@"} {
		if strings.Contains(publishJob, forbidden) {
			t.Fatalf("publish job contains a signing secret or source execution %q", forbidden)
		}
	}
	if !strings.Contains(publishJob, "PROMOTION_PUSH_TOKEN") ||
		!strings.Contains(publishJob, "contents: write") {
		t.Fatal("publish job does not isolate exact write authority")
	}
	verifyIndex := strings.Index(source, `--mode historical`)
	pushIndex := strings.Index(source, `git push --atomic`)
	if verifyIndex < 0 || pushIndex <= verifyIndex {
		t.Fatalf("workflow order verify=%d push=%d", verifyIndex, pushIndex)
	}
	if strings.Contains(source, "persist-credentials: true") {
		t.Fatal("promotion workflow persists a write credential into a checkout")
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
