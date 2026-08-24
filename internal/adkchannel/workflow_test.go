package adkchannel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type receiverWorkflow struct {
	On          map[string]receiverEvent `yaml:"on"`
	Permissions map[string]string        `yaml:"permissions"`
	Jobs        map[string]receiverJob   `yaml:"jobs"`
}

type receiverEvent struct {
	Inputs map[string]receiverInput `yaml:"inputs"`
}

type receiverInput struct {
	Required bool   `yaml:"required"`
	Type     string `yaml:"type"`
}

type receiverJob struct {
	If          string            `yaml:"if"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []receiverStep    `yaml:"steps"`
}

type receiverStep struct {
	Run string `yaml:"run"`
}

func readReceiverWorkflow(t *testing.T) ([]byte, receiverWorkflow) {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "adk-channel-receive.yml")
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

func TestReceiverWorkflowDispatchContractAndPermissions(t *testing.T) {
	contents, workflow := readReceiverWorkflow(t)
	if len(workflow.On) != 1 {
		t.Fatalf("triggers = %v, want workflow_dispatch only", workflow.On)
	}
	dispatch, ok := workflow.On["workflow_dispatch"]
	if !ok {
		t.Fatalf("triggers = %v, want workflow_dispatch", workflow.On)
	}
	if len(dispatch.Inputs) != 2 {
		t.Fatalf("inputs = %v, want exactly two", dispatch.Inputs)
	}
	for _, name := range []string{"document_base64", "signature_base64"} {
		input, ok := dispatch.Inputs[name]
		if !ok || !input.Required || input.Type != "string" {
			t.Fatalf("input %q = %+v, want required string", name, input)
		}
	}
	if len(workflow.Permissions) != 0 {
		t.Fatalf("top-level permissions = %v, want empty", workflow.Permissions)
	}
	if len(workflow.Jobs) != 1 {
		t.Fatalf("jobs = %v, want receive only", workflow.Jobs)
	}
	job, ok := workflow.Jobs["receive"]
	if !ok {
		t.Fatal("receive job is missing")
	}
	if len(job.Permissions) != 1 || job.Permissions["contents"] != "write" {
		t.Fatalf("receive permissions = %v, want contents: write only", job.Permissions)
	}
	if !strings.Contains(job.If, "default_branch") {
		t.Fatalf("receive job guard = %q, want default branch binding", job.If)
	}

	text := string(contents)
	if strings.Count(text, "contents: write") != 1 {
		t.Fatal("contents: write must occur only on the receive job")
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "private_key") || strings.Contains(lower, "private-key") || strings.Contains(text, "secrets.") {
		t.Fatal("receiver workflow must not access signing key material or repository secrets")
	}
	for _, required := range []string{
		"vars.AUTOPUS_ADK_CHANNEL_KEY_ID",
		"vars.AUTOPUS_ADK_CHANNEL_PUBLIC_KEY",
		"inputs.document_base64",
		"inputs.signature_base64",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("workflow is missing %q", required)
		}
	}
}

func TestReceiverWorkflowPublicationCapabilityBoundary(t *testing.T) {
	contents, workflow := readReceiverWorkflow(t)
	job := workflow.Jobs["receive"]
	var runs []string
	for _, step := range job.Steps {
		if step.Run != "" {
			runs = append(runs, step.Run)
		}
	}
	script := strings.Join(runs, "\n")
	text := string(contents)

	for _, forbidden := range []string{"--force", "force-with-lease", "+refs/heads/", "|| true"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("workflow contains forbidden publication capability %q", forbidden)
		}
	}
	if strings.Count(script, " push origin ") != 1 ||
		!strings.Contains(script, `"HEAD:refs/heads/release-channel"`) {
		t.Fatal("workflow must push one exact release-channel refspec")
	}
	for _, required := range []string{
		`"refs/heads/release-channel:refs/remotes/origin/release-channel"`,
		`--current-document "$channel_root/adk-channel.json"`,
		`--current-signature "$channel_root/adk-channel.json.sig"`,
		`checkout -q --orphan release-channel`,
		`adk-channel.json`,
		`adk-channel.json.sig`,
		`cat-file blob :adk-channel.json`,
		`cat-file blob :adk-channel.json.sig`,
		`cmp -s -- "$incoming/adk-channel.json" "$staged_root/adk-channel.json"`,
		`env -u GITHUB_TOKEN "$verifier" verify`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("publication script is missing %q", required)
		}
	}
	if !strings.Contains(text, `ADK_CHANNEL_DOCUMENT_BASE64: ${{ inputs.document_base64 }}`) ||
		!strings.Contains(text, `ADK_CHANNEL_SIGNATURE_BASE64: ${{ inputs.signature_base64 }}`) {
		t.Fatal("dispatch bytes must cross only the decoder environment boundary")
	}

	preVerify := strings.Index(script, `"$verifier" verify-update`)
	copyDocument := strings.Index(script, `cp -- "$incoming/adk-channel.json" "$channel_root/adk-channel.json"`)
	stage := strings.Index(script, `/usr/bin/git -C "$channel_root" add`)
	reverify := strings.LastIndex(script, `"$verifier" verify`)
	commit := strings.Index(script, `/usr/bin/git -C "$channel_root" commit`)
	push := strings.Index(script, `/usr/bin/git -C "$channel_root" push`)
	if preVerify < 0 || copyDocument < 0 || stage < 0 || reverify < 0 || commit < 0 || push < 0 ||
		!(preVerify < copyDocument && copyDocument < stage && stage < reverify &&
			reverify < commit && commit < push) {
		t.Fatalf("unsafe publication order: pre=%d copy=%d stage=%d reverify=%d commit=%d push=%d",
			preVerify, copyDocument, stage, reverify, commit, push)
	}
}
