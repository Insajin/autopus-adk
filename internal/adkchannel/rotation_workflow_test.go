package adkchannel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readRotationWorkflow(t *testing.T) ([]byte, receiverWorkflow) {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "adk-key-rotation-receive.yml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflow receiverWorkflow
	if err := yaml.Unmarshal(contents, &workflow); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	return contents, workflow
}

func TestRotationWorkflowIsManualMainOnlyWithoutSigningAuthority(t *testing.T) {
	contents, workflow := readRotationWorkflow(t)
	if len(workflow.On) != 1 {
		t.Fatalf("triggers = %v, want workflow_dispatch only", workflow.On)
	}
	dispatch, ok := workflow.On["workflow_dispatch"]
	if !ok || len(dispatch.Inputs) != 2 {
		t.Fatalf("workflow_dispatch = %+v, want two inputs", dispatch)
	}
	for _, name := range []string{"document_base64", "signature_base64"} {
		input := dispatch.Inputs[name]
		if !input.Required || input.Type != "string" {
			t.Fatalf("input %q = %+v, want required string", name, input)
		}
	}
	if len(workflow.Permissions) != 0 || len(workflow.Jobs) != 1 {
		t.Fatalf("permissions/jobs = (%v, %v)", workflow.Permissions, workflow.Jobs)
	}
	job := workflow.Jobs["receive"]
	if len(job.Permissions) != 1 || job.Permissions["contents"] != "read" {
		t.Fatalf("receive permissions = %v, want contents: read only", job.Permissions)
	}
	if !strings.Contains(job.If, "default_branch") || !strings.Contains(job.If, "refs/heads/main") {
		t.Fatalf("receive guard = %q, want exact default main", job.If)
	}

	text := string(contents)
	lower := strings.ToLower(text)
	for _, forbidden := range []string{
		"secrets.", "private_key", "private-key", "environment:",
		"pull_request:", "repository_dispatch:", "schedule:", "ssh-keygen -y sign",
		"openssl dgst -sign", "cosign sign",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("workflow contains forbidden authority %q", forbidden)
		}
	}
}

func TestRotationWorkflowAuditsFixedImmutableSidecarWithoutMutation(t *testing.T) {
	contents, workflow := readRotationWorkflow(t)
	job := workflow.Jobs["receive"]
	var runs []string
	for _, step := range job.Steps {
		if step.Run != "" {
			runs = append(runs, step.Run)
		}
	}
	script := strings.Join(runs, "\n")
	text := string(contents)
	for _, required := range []string{
		"inputs.document_base64",
		"inputs.signature_base64",
		"adk-key-rotation-v1.json",
		"adk-key-rotation-v1.sig",
		"refs/heads/release-key-rotation-v0.50.109",
		"release-tag-signing-2026-q3-r2.pub",
		"release-tag-signing-2026-q3-r2.fingerprint",
		"omp-context-promotion-2026-q3-k3.pub",
		"materialize-key-rotation-authority.sh",
		"key-rotation-authority/verify-rotation.sh",
		"verify-rotation-ref-ruleset.sh",
		"openssl base64 -d -A",
		"verify-rotation",
		`source_commit="$(/usr/bin/git rev-parse --verify HEAD)"`,
		`source_tree="$(/usr/bin/git rev-parse --verify 'HEAD^{tree}')"`,
		`fetch --no-tags origin "$rotation_ref"`,
		`rev-list --parents -n 1 "$rotation_commit"`,
		`ls-tree -r "$rotation_commit"`,
		`cat-file blob "$rotation_commit:adk-key-rotation-v1.json"`,
		`cat-file blob "$rotation_commit:adk-key-rotation-v1.sig"`,
		`cmp -s -- "$incoming/adk-key-rotation-v1.json" "$audited/adk-key-rotation-v1.json"`,
		`cmp -s -- "$incoming/adk-key-rotation-v1.sig" "$audited/adk-key-rotation-v1.sig"`,
		"100644",
		"env -u GITHUB_TOKEN",
		"GITHUB_STEP_SUMMARY",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("workflow is missing %q", required)
		}
	}
	if strings.Contains(text, "adk-key-rotation-v1.json.sig") {
		t.Fatal("workflow must use the fixed adk-key-rotation-v1.sig filename")
	}
	for _, forbidden := range []string{
		"contents: write", "credential.helper", " push ", "git commit", "git add",
		"checkout -q --orphan", "--force", "force-with-lease", "+refs/heads/",
		"verify-rotation-historical", "secrets.",
		"internal/adkchannel/cmd",
	} {
		if strings.Contains(script, forbidden) || strings.Contains(text, forbidden) {
			t.Fatalf("read-only auditor contains forbidden mutation %q", forbidden)
		}
	}

	fetch := strings.Index(script, `fetch --no-tags origin "$rotation_ref"`)
	extract := strings.Index(script, `cat-file blob "$rotation_commit:adk-key-rotation-v1.json"`)
	compare := strings.Index(script, `cmp -s -- "$incoming/adk-key-rotation-v1.json"`)
	verify := strings.LastIndex(script, "verify-rotation")
	summary := strings.Index(script, "GITHUB_STEP_SUMMARY")
	if fetch < 0 || extract < 0 || compare < 0 || verify < 0 || summary < 0 ||
		!(fetch < extract && extract < compare && compare < verify && verify < summary) {
		t.Fatalf("unsafe audit order: fetch=%d extract=%d compare=%d verify=%d summary=%d",
			fetch, extract, compare, verify, summary)
	}
}
