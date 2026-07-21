package companionmanifest

import (
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type publicKeyReceiptArchiveConfig struct {
	Archives []struct {
		ID    string `yaml:"id"`
		Files []struct {
			Source      string `yaml:"src"`
			Destination string `yaml:"dst"`
		} `yaml:"files"`
	} `yaml:"archives"`
}

type publicKeyReceiptWorkflow struct {
	Env  map[string]string                      `yaml:"env"`
	Jobs map[string]publicKeyReceiptWorkflowJob `yaml:"jobs"`
}

type publicKeyReceiptWorkflowJob struct {
	Uses  string                         `yaml:"uses"`
	Env   map[string]string              `yaml:"env"`
	Steps []publicKeyReceiptWorkflowStep `yaml:"steps"`
}

type publicKeyReceiptWorkflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	If   string            `yaml:"if"`
	Run  string            `yaml:"run"`
	Env  map[string]string `yaml:"env"`
	With map[string]any    `yaml:"with"`
}

type publicKeyReceiptArtifactAPI struct {
	flag     string
	artifact string
	bundle   bool
}

func TestReleasePublicKeyReceipt_GoReleaserDarwinArchive_MatchesAtomicCLIArtifact(t *testing.T) {
	api := productionPublicKeyReceiptAPI(t)
	var config publicKeyReceiptArchiveConfig
	if err := yaml.Unmarshal(readRepositoryFile(t, ".goreleaser.yaml"), &config); err != nil {
		t.Fatalf("parse GoReleaser public-key receipt contract: %v", err)
	}
	var mappings []struct {
		Source      string
		Destination string
	}
	for _, archive := range config.Archives {
		if archive.ID != "auto" {
			continue
		}
		for _, file := range archive.Files {
			if strings.Contains(file.Source, "public-key-receipt") ||
				strings.Contains(file.Destination, "public-key-receipt") {
				mappings = append(mappings, struct {
					Source      string
					Destination string
				}{file.Source, file.Destination})
			}
		}
	}
	if len(mappings) != 1 {
		t.Fatalf("missing production contract: Darwin archive must map exactly one atomic %s artifact, got %#v", api.artifact, mappings)
	}
	mapping := mappings[0]
	for _, required := range []string{
		`{{ if eq .Os "darwin" }}`,
		"dist/auto_{{ .Target }}/" + api.artifact,
		`scripts/companion-release/no-files-*`,
	} {
		if !strings.Contains(mapping.Source, required) {
			t.Fatalf("Darwin public-key receipt source %q does not match secure CLI %s contract: missing %q", mapping.Source, api.flag, required)
		}
	}
	if mapping.Destination != api.artifact {
		t.Fatalf("Darwin public-key receipt destination = %q, want exact atomic artifact %q", mapping.Destination, api.artifact)
	}
	if api.bundle && !strings.Contains(mapping.Source, api.artifact+"/**") {
		t.Fatalf("receipt bundle mapping %q must archive the one published bundle namespace", mapping.Source)
	}
	for _, unsafe := range []string{"public-key-receipt.json", "public-key-receipt.sig"} {
		if mapping.Destination == unsafe || strings.HasSuffix(mapping.Source, "/"+unsafe) {
			t.Fatalf("unsafe sibling %q is independently claimed as crash-atomic", unsafe)
		}
	}
}

func TestReleasePublicKeyReceipt_Producer_UsesSameKeyWithoutLeakAndSurfacesRollback(t *testing.T) {
	api := productionPublicKeyReceiptAPI(t)
	producer := normalizedReleaseText(releaseProducerSurface(t))
	if !strings.Contains(producer, "companion-manifest public-key-receipt") {
		t.Fatal("missing production contract: produce.sh never invokes the public-key-receipt CLI")
	}
	manifestKey := `<"$COMPANION_SIGNING_KEY_FILE"`
	receiptKey := `--key-file "$COMPANION_SIGNING_KEY_FILE"`
	if !strings.Contains(producer, manifestKey) || !strings.Contains(producer, receiptKey) {
		t.Fatalf("manifest and receipt must consume the same COMPANION_SIGNING_KEY_FILE: manifest=%v receipt=%v", strings.Contains(producer, manifestKey), strings.Contains(producer, receiptKey))
	}
	if !strings.Contains(producer, api.flag+` "$`) || !strings.Contains(producer, api.artifact) {
		t.Fatalf("produce.sh public-key receipt output does not match CLI %s / %s", api.flag, api.artifact)
	}
	for _, forbidden := range []string{
		"set -x", "echo $COMPANION_SIGNING_KEY_FILE", "printf $COMPANION_SIGNING_KEY_FILE",
		`cat "$COMPANION_SIGNING_KEY_FILE"`, `$(<"$COMPANION_SIGNING_KEY_FILE")`,
		`$(cat "$COMPANION_SIGNING_KEY_FILE")`, "printenv", "env |",
	} {
		if strings.Contains(producer, forbidden) {
			t.Fatalf("private signing material can leak through producer token %q", forbidden)
		}
	}
	for _, required := range []string{
		api.artifact,
		"public key receipt production failed",
		"rollback_status",
		"partial release publication rollback failed",
		"trap cleanup EXIT",
	} {
		if !strings.Contains(producer, required) {
			t.Fatalf("missing production rollback contract %q for atomic receipt publication", required)
		}
	}
	if strings.Contains(producer, "set +e") || strings.Contains(producer, "|| true") {
		t.Fatal("producer suppresses a publication/rollback failure instead of surfacing partial state")
	}
}

func productionPublicKeyReceiptAPI(t *testing.T) publicKeyReceiptArtifactAPI {
	t.Helper()
	source := string(releaseSourceFile(t, "internal/cli/companion_public_key_receipt.go"))
	bundle := strings.Contains(source, `"bundle-output"`)
	envelope := strings.Contains(source, `"envelope-output"`)
	if bundle == envelope {
		t.Fatalf("secure public-key receipt API must expose exactly one atomic bundle or envelope output: bundle=%v envelope=%v", bundle, envelope)
	}
	if strings.Contains(source, `"receipt-output"`) || strings.Contains(source, `"signature-output"`) {
		t.Fatal("secure public-key receipt API exposes independently published receipt/signature siblings")
	}
	if bundle {
		return publicKeyReceiptArtifactAPI{
			flag: "--bundle-output", artifact: "adk-companion-public-key-receipt.bundle", bundle: true,
		}
	}
	return publicKeyReceiptArtifactAPI{
		flag: "--envelope-output", artifact: "adk-companion-public-key-receipt.envelope",
	}
}

func releaseWorkflowContract(t *testing.T) publicKeyReceiptWorkflow {
	t.Helper()
	var workflow publicKeyReceiptWorkflow
	if err := yaml.Unmarshal(releaseSourceFile(t, filepath.Join(".github", "workflows", "release.yaml")), &workflow); err != nil {
		t.Fatalf("parse release workflow: %v", err)
	}
	return workflow
}

func releaseSourceFile(t *testing.T, name string) []byte {
	t.Helper()
	return readRepositoryFile(t, name)
}

func normalizedReleaseText(data []byte) string {
	return strings.Join(strings.Fields(string(data)), " ")
}

func releaseWorkflowStepContaining(t *testing.T, workflow publicKeyReceiptWorkflow, token string) (int, publicKeyReceiptWorkflowStep) {
	t.Helper()
	job, ok := workflow.Jobs["release"]
	if !ok {
		t.Fatal("release workflow has no release job")
	}
	for index, step := range job.Steps {
		if strings.Contains(strings.ToLower(step.Name+" "+step.Run), strings.ToLower(token)) {
			return index, step
		}
	}
	t.Fatalf("release workflow has no step containing %q", token)
	return -1, publicKeyReceiptWorkflowStep{}
}
