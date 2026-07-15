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
		Command   string `yaml:"cmd"`
		Artifacts string `yaml:"artifacts"`
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
	if len(config.Signs) != 1 || config.Signs[0].Command != "cosign" || config.Signs[0].Artifacts != "checksum" {
		t.Fatalf("cosign checksum flow changed: %#v", config.Signs)
	}
}

func TestReleaseWorkflow_RequiresRealKeyAndMetadataBeforeGoReleaser(t *testing.T) {
	workflow := string(readRepositoryFile(t, filepath.Join(".github", "workflows", "release.yaml")))
	required := []string{
		"runs-on: macos-14",
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
