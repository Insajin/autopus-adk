package companionmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseConfig struct {
	Builds []struct {
		ID     string   `yaml:"id"`
		Goos   []string `yaml:"goos"`
		Goarch []string `yaml:"goarch"`
		Hooks  struct {
			Post []struct {
				Command string   `yaml:"cmd"`
				Env     []string `yaml:"env"`
			} `yaml:"post"`
		} `yaml:"hooks"`
	} `yaml:"builds"`
	Archives []struct {
		ID    string `yaml:"id"`
		Files []struct {
			Source      string `yaml:"src"`
			Destination string `yaml:"dst"`
		} `yaml:"files"`
	} `yaml:"archives"`
	HomebrewCasks []struct {
		Name      string   `yaml:"name"`
		IDs       []string `yaml:"ids"`
		Directory string   `yaml:"directory"`
		Binaries  []string `yaml:"binaries"`
	} `yaml:"homebrew_casks"`
	Signs []struct {
		ID        string   `yaml:"id"`
		Command   string   `yaml:"cmd"`
		Artifacts string   `yaml:"artifacts"`
		Arguments []string `yaml:"args"`
		Signature string   `yaml:"signature"`
		Output    bool     `yaml:"output"`
	} `yaml:"signs"`
}

func TestGoReleaser_CompanionProducerIsAssociatedWithEveryDarwinBuild(t *testing.T) {
	data := readRepositoryFile(t, ".goreleaser.yaml")
	var config releaseConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	build := findReleaseBuild(t, config, "auto")
	for _, value := range []string{"linux", "darwin", "windows"} {
		assertContains(t, build.Goos, value)
	}
	for _, value := range []string{"amd64", "arm64"} {
		assertContains(t, build.Goarch, value)
	}
	if len(build.Hooks.Post) != 1 || build.Hooks.Post[0].Command != "scripts/companion-release/produce.sh" {
		t.Fatalf("auto post hooks = %#v", build.Hooks.Post)
	}
	environment := strings.Join(build.Hooks.Post[0].Env, "\n")
	for _, binding := range []string{
		"COMPANION_ARTIFACT={{ .Path }}",
		"COMPANION_TARGET={{ .Target }}",
		"COMPANION_PLATFORM={{ .Os }}",
		"COMPANION_ARCHITECTURE={{ .Arch }}",
		"COMPANION_VERSION={{ .Version }}",
	} {
		if !strings.Contains(environment, binding) {
			t.Fatalf("post-hook environment missing %q", binding)
		}
	}
	archive := findReleaseArchive(t, config, "auto")
	for _, name := range []string{
		"adk-companion-manifest.json",
		"adk-companion-manifest.sig",
		"adk-companion-darwin-receipt.json",
	} {
		if !archiveMapsCompanionFile(archive.Files, name) {
			t.Fatalf("archive does not map exact companion file %q: %#v", name, archive.Files)
		}
	}
	if len(config.HomebrewCasks) != 1 {
		t.Fatalf("Homebrew cask count = %d", len(config.HomebrewCasks))
	}
	cask := config.HomebrewCasks[0]
	if cask.Name != "auto" || cask.Directory != "Casks" ||
		len(cask.IDs) != 1 || cask.IDs[0] != "auto" ||
		len(cask.Binaries) != 1 || cask.Binaries[0] != "auto" {
		t.Fatalf("Homebrew cask flow changed: %#v", cask)
	}
	if len(config.Signs) != 2 || config.Signs[0].Command != "cosign" || config.Signs[0].Artifacts != "checksum" {
		t.Fatalf("cosign checksum flow changed: %#v", config.Signs)
	}
	if config.Signs[1].ID != "ecdsa-release-envelope" ||
		config.Signs[1].Command != "scripts/release-signing/sign-checksums.sh" ||
		config.Signs[1].Artifacts != "checksum" ||
		config.Signs[1].Signature != "${artifact}.signatures" ||
		!config.Signs[1].Output {
		t.Fatalf("ECDSA release-signing checksum flow changed: %#v", config.Signs)
	}
	wantArguments := []string{
		"${artifact}",
		"${signature}",
		"{{ .Env.ADK_RELEASE_ECDSA_PRIVATE_KEY_FILE }}",
	}
	if strings.Join(config.Signs[1].Arguments, "\n") != strings.Join(wantArguments, "\n") {
		t.Fatalf("ECDSA release-signing arguments = %#v, want %#v", config.Signs[1].Arguments, wantArguments)
	}
}

func TestReleaseWorkflow_RequiresRealKeyAndMetadataBeforeGoReleaser(t *testing.T) {
	workflow := string(readRepositoryFile(t, filepath.Join(".github", "workflows", "release.yaml")))
	var permissionContract struct {
		Jobs map[string]struct {
			Permissions map[string]string `yaml:"permissions"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflow), &permissionContract); err != nil {
		t.Fatalf("parse release workflow permissions: %v", err)
	}
	if got := permissionContract.Jobs["release"].Permissions["id-token"]; got != "write" {
		t.Fatalf("release job id-token permission = %q, want write", got)
	}
	required := []string{
		"runs-on: macos-15",
		"secrets.APPLE_API_ISSUER",
		"secrets.APPLE_API_KEY",
		"secrets.APPLE_API_KEY_P8",
		"secrets.APPLE_CERTIFICATE",
		"secrets.APPLE_CERTIFICATE_PASSWORD",
		"secrets.ADK_COMPANION_ED25519_PRIVATE_KEY",
		"vars.ADK_COMPANION_KEY_ID",
		"vars.ADK_COMPANION_ROLLBACK_FLOOR",
		"vars.ADK_COMPANION_HANDOFF",
		"vars.ADK_COMPANION_ISSUED_AT",
		"vars.ADK_COMPANION_EXPIRES_AT",
		"COMPANION_BUILD_PROVENANCE:",
		"COMPANION_SIGNING_KEY_FILE=",
		"COMPANION_RELEASE_PRODUCTION:",
		"scripts/companion-release/validate-environment.sh",
		"if: always()",
		"security list-keychains -d user -s \"$keychain_path\"",
		"security delete-keychain",
	}
	for _, value := range required {
		if !strings.Contains(workflow, value) {
			t.Fatalf("release workflow missing %q", value)
		}
	}
	validateIndex := strings.Index(workflow, "validate-environment.sh")
	releaseIndex := strings.Index(workflow, "goreleaser release --clean")
	if validateIndex < 0 || releaseIndex < 0 || validateIndex >= releaseIndex {
		t.Fatal("companion release validation must run before GoReleaser")
	}
	_, goReleaserStep := releaseWorkflowStepContaining(t, releaseWorkflowContract(t), "goreleaser release --clean")
	if !goReleaserOIDCCommandValid(goReleaserStep.Run) {
		t.Fatal("GoReleaser OIDC bindings must be arguments of its continued env -i command")
	}
	mutations := map[string]string{
		"continuation_removed_before_oidc": strings.Replace(goReleaserStep.Run,
			`GITHUB_REF_NAME="$GITHUB_REF_NAME" GITHUB_TOKEN="$GITHUB_TOKEN" \`,
			`GITHUB_REF_NAME="$GITHUB_REF_NAME" GITHUB_TOKEN="$GITHUB_TOKEN"`, 1),
		"oidc_binding_outside_command": strings.Replace(goReleaserStep.Run,
			`ACTIONS_ID_TOKEN_REQUEST_TOKEN="$ACTIONS_ID_TOKEN_REQUEST_TOKEN" \`,
			`ACTIONS_ID_TOKEN_REQUEST_TOKEN="$ACTIONS_ID_TOKEN_REQUEST_TOKEN"`, 1),
	}
	for name, mutated := range mutations {
		if mutated == goReleaserStep.Run {
			t.Fatalf("mutation %s did not alter GoReleaser command", name)
		}
		if goReleaserOIDCCommandValid(mutated) {
			t.Fatalf("invalid GoReleaser command passed contract mutation %s", name)
		}
	}
	importIndex := strings.Index(workflow, "security import")
	partitionIndex := strings.Index(workflow, "security set-key-partition-list")
	searchListIndex := strings.Index(workflow, "security list-keychains -d user -s")
	identityIndex := strings.Index(workflow, "security find-identity")
	if importIndex < 0 || partitionIndex <= importIndex || searchListIndex <= partitionIndex ||
		identityIndex <= searchListIndex {
		t.Fatal("temporary signing keychain must enter the user search list before identity lookup")
	}
	releaseStep := workflow[releaseIndex:]
	if strings.Contains(releaseStep, "ADK_COMPANION_ED25519_PRIVATE_KEY") {
		t.Fatal("GoReleaser step receives private key instead of key-file path")
	}
}

func goReleaserOIDCCommandValid(run string) bool {
	var command strings.Builder
	started := false
	terminalLine := ""
	for _, rawLine := range strings.Split(run, "\n") {
		line := strings.TrimSpace(rawLine)
		if !started {
			if !strings.HasPrefix(line, "env -i ") {
				continue
			}
			started = true
		}
		continued := strings.HasSuffix(line, `\`)
		line = strings.TrimSpace(strings.TrimSuffix(line, `\`))
		command.WriteString(line + " ")
		if !continued {
			terminalLine = line
			break
		}
	}
	if terminalLine != "goreleaser release --clean" {
		return false
	}
	for _, name := range []string{"ACTIONS_ID_TOKEN_REQUEST_TOKEN", "ACTIONS_ID_TOKEN_REQUEST_URL"} {
		if !strings.Contains(command.String(), name+`="$`+name+`"`) {
			return false
		}
	}
	return true
}

func readRepositoryFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findReleaseBuild(t *testing.T, config releaseConfig, id string) *struct {
	ID     string   `yaml:"id"`
	Goos   []string `yaml:"goos"`
	Goarch []string `yaml:"goarch"`
	Hooks  struct {
		Post []struct {
			Command string   `yaml:"cmd"`
			Env     []string `yaml:"env"`
		} `yaml:"post"`
	} `yaml:"hooks"`
} {
	t.Helper()
	for index := range config.Builds {
		if config.Builds[index].ID == id {
			return &config.Builds[index]
		}
	}
	t.Fatalf("release build %q not found", id)
	return nil
}

func findReleaseArchive(t *testing.T, config releaseConfig, id string) *struct {
	ID    string `yaml:"id"`
	Files []struct {
		Source      string `yaml:"src"`
		Destination string `yaml:"dst"`
	} `yaml:"files"`
} {
	t.Helper()
	for index := range config.Archives {
		if config.Archives[index].ID == id {
			return &config.Archives[index]
		}
	}
	t.Fatalf("release archive %q not found", id)
	return nil
}

func archiveMapsCompanionFile(files []struct {
	Source      string `yaml:"src"`
	Destination string `yaml:"dst"`
}, name string) bool {
	want := `{{ if eq .Os "darwin" }}dist/auto_{{ .Target }}/` + name +
		`{{ else }}scripts/companion-release/no-files-*{{ end }}`
	for _, file := range files {
		if file.Source == want && file.Destination == name {
			return true
		}
	}
	return false
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %v", want, values)
}
