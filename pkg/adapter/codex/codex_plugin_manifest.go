package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type pluginManifest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Author      pluginAuthor    `json:"author"`
	Homepage    string          `json:"homepage"`
	Repository  string          `json:"repository"`
	License     string          `json:"license"`
	Keywords    []string        `json:"keywords"`
	Skills      string          `json:"skills"`
	Interface   pluginInterface `json:"interface"`
}

type pluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

type pluginInterface struct {
	DisplayName       string   `json:"displayName"`
	ShortDescription  string   `json:"shortDescription"`
	LongDescription   string   `json:"longDescription"`
	DeveloperName     string   `json:"developerName"`
	Category          string   `json:"category"`
	Capabilities      []string `json:"capabilities"`
	WebsiteURL        string   `json:"websiteURL"`
	PrivacyPolicyURL  string   `json:"privacyPolicyURL"`
	TermsOfServiceURL string   `json:"termsOfServiceURL"`
	DefaultPrompt     []string `json:"defaultPrompt"`
	BrandColor        string   `json:"brandColor"`
}

type marketplaceDoc struct {
	Name      string             `json:"name"`
	Interface marketplaceUI      `json:"interface,omitempty"`
	Plugins   []marketplaceEntry `json:"plugins"`
}

type marketplaceUI struct {
	DisplayName string `json:"displayName,omitempty"`
}

type marketplaceEntry struct {
	Name     string            `json:"name"`
	Source   marketplaceSource `json:"source"`
	Policy   marketplacePolicy `json:"policy"`
	Category string            `json:"category"`
}

type marketplaceSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type marketplacePolicy struct {
	Installation   string   `json:"installation"`
	Authentication string   `json:"authentication"`
	Products       []string `json:"products,omitempty"`
}

func (a *Adapter) renderPluginManifestJSON() (string, error) {
	doc := pluginManifest{
		Name:        "auto",
		Version:     "1.0.0",
		Description: "Autopus workflow router for Codex: plan, go, fix, review, sync, canary, and idea.",
		Author:      pluginAuthor{Name: "Autopus", Email: "noreply@autopus.co", URL: "https://autopus.co"},
		Homepage:    "https://autopus.co",
		Repository:  "https://github.com/insajin/autopus-adk",
		License:     "Apache-2.0",
		Keywords:    []string{"autopus", "workflow", "planning", "codex", "multi-provider"},
		Skills:      "./skills",
		Interface: pluginInterface{
			DisplayName:       "Auto",
			ShortDescription:  "Autopus workflow router for Codex",
			LongDescription:   "Run Autopus plan/go/fix/review/sync/canary/idea workflows from Codex with a local plugin plus repository-managed helper docs.",
			DeveloperName:     "Autopus",
			Category:          "Developer Tools",
			Capabilities:      []string{"Interactive", "Write", "Planning"},
			WebsiteURL:        "https://autopus.co",
			PrivacyPolicyURL:  "https://autopus.co/privacy",
			TermsOfServiceURL: "https://autopus.co/terms",
			DefaultPrompt: []string{
				"@auto plan \"새 기능 요구사항을 SPEC으로 정리해줘\"",
				"@auto go SPEC-EXAMPLE-001",
				"@auto idea \"새 워크플로우를 멀티 프로바이더로 토론해줘\" --multi",
			},
			BrandColor: "#0F766E",
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("plugin.json 직렬화 실패: %w", err)
	}
	return string(data) + "\n", nil
}

func (a *Adapter) renderMarketplaceJSON() (string, error) {
	doc := marketplaceDoc{
		Name:      "autopus-local",
		Interface: marketplaceUI{DisplayName: "Autopus Local"},
		Plugins: []marketplaceEntry{{
			Name:     "auto",
			Source:   marketplaceSource{Source: "local", Path: "./.autopus/plugins/auto"},
			Policy:   marketplacePolicy{Installation: "AVAILABLE", Authentication: "ON_INSTALL"},
			Category: "Developer Tools",
		}},
	}

	existingPath := filepath.Join(a.root, ".agents", "plugins", "marketplace.json")
	if data, err := os.ReadFile(existingPath); err == nil {
		var existing marketplaceDoc
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			if existing.Name != "" {
				doc.Name = existing.Name
			}
			if existing.Interface.DisplayName != "" {
				doc.Interface.DisplayName = existing.Interface.DisplayName
			}
			updated := false
			for i := range existing.Plugins {
				if existing.Plugins[i].Name == "auto" {
					existing.Plugins[i] = doc.Plugins[0]
					updated = true
					break
				}
			}
			if !updated {
				existing.Plugins = append(existing.Plugins, doc.Plugins[0])
			}
			doc.Plugins = existing.Plugins
		}
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marketplace.json 직렬화 실패: %w", err)
	}
	return string(data) + "\n", nil
}
