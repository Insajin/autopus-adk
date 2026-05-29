package design

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type FigmaRef struct {
	SourcePath string `json:"source_path"`
	Kind       string `json:"kind"`
	URL        string `json:"url"`
	URLHash    string `json:"url_hash"`
	FileKey    string `json:"file_key,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
}

type CodeConnectAudit struct {
	Status      string      `json:"status"`
	MappingRefs []SourceRef `json:"mapping_refs,omitempty"`
	PackageRefs []SourceRef `json:"package_refs,omitempty"`
	SetupGaps   []string    `json:"setup_gaps,omitempty"`
}

type FigmaAudit struct {
	FigmaRefs   []FigmaRef       `json:"figma_refs,omitempty"`
	CodeConnect CodeConnectAudit `json:"code_connect"`
	SetupGaps   []string         `json:"setup_gaps,omitempty"`
}

var figmaURLPattern = regexp.MustCompile(`https://(?:www\.)?figma\.com/(?:design|file|proto|board|figjam)/[^\s<>\]\)'"` + "`" + `]+`)

func AuditFigma(root string, maxRefs int) (FigmaAudit, error) {
	if maxRefs <= 0 {
		maxRefs = 30
	}
	var refs []FigmaRef
	var mappingRefs []SourceRef
	var packageRefs []SourceRef

	err := walkDesignCandidateFiles(root, maxRefs*4, func(rel, abs string, info os.FileInfo) error {
		base := strings.ToLower(filepath.Base(rel))
		ext := strings.ToLower(filepath.Ext(rel))
		if isCodeConnectMappingPath(rel) {
			mappingRefs = appendLimited(mappingRefs, SourceRef{Path: rel, Kind: "code_connect_mapping"}, maxRefs)
		}
		if base == "package.json" {
			data, err := os.ReadFile(abs)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "@figma/code-connect") {
				packageRefs = appendLimited(packageRefs, SourceRef{Path: rel, Kind: "package_dependency"}, maxRefs)
			}
		}
		if ext == ".md" || ext == ".mdx" || ext == ".txt" || base == "design.md" || base == "readme.md" {
			data, err := os.ReadFile(abs)
			if err != nil {
				return err
			}
			for _, ref := range ExtractFigmaRefs(rel, string(data)) {
				if len(refs) >= maxRefs {
					break
				}
				refs = append(refs, ref)
			}
		}
		return nil
	})
	if err != nil {
		return FigmaAudit{}, err
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].SourcePath == refs[j].SourcePath {
			return refs[i].URL < refs[j].URL
		}
		return refs[i].SourcePath < refs[j].SourcePath
	})
	sortRefs(mappingRefs)
	sortRefs(packageRefs)

	status := "missing"
	var codeConnectGaps []string
	if len(mappingRefs) > 0 || len(packageRefs) > 0 {
		status = "detected"
	} else {
		codeConnectGaps = append(codeConnectGaps, "code_connect_mapping_missing")
	}
	gaps := append([]string{}, codeConnectGaps...)
	if len(refs) == 0 {
		gaps = append(gaps, "figma_reference_missing")
	}
	return FigmaAudit{
		FigmaRefs: refs,
		CodeConnect: CodeConnectAudit{
			Status:      status,
			MappingRefs: mappingRefs,
			PackageRefs: packageRefs,
			SetupGaps:   codeConnectGaps,
		},
		SetupGaps: gaps,
	}, nil
}

func ExtractFigmaRefs(sourcePath, content string) []FigmaRef {
	matches := figmaURLPattern.FindAllString(content, -1)
	seen := map[string]bool{}
	var refs []FigmaRef
	for _, raw := range matches {
		clean, kind, fileKey, nodeID := parseFigmaURL(raw)
		identity := clean
		if nodeID != "" {
			identity += "#" + nodeID
		}
		if clean == "" || seen[identity] {
			continue
		}
		seen[identity] = true
		refs = append(refs, FigmaRef{
			SourcePath: sourcePath,
			Kind:       kind,
			URL:        clean,
			URLHash:    shortHash(identity),
			FileKey:    fileKey,
			NodeID:     nodeID,
		})
	}
	return refs
}

func (a FigmaAudit) Markdown() string {
	var sb strings.Builder
	sb.WriteString("## Figma And Code Connect Audit\n")
	if len(a.FigmaRefs) == 0 {
		sb.WriteString("- Figma refs: none\n")
	} else {
		sb.WriteString("- Figma refs:\n")
		for _, ref := range a.FigmaRefs {
			fmt.Fprintf(&sb, "  - %s (%s, %s)\n", ref.SourcePath, ref.Kind, ref.URLHash)
		}
	}
	fmt.Fprintf(&sb, "- Code Connect: %s\n", a.CodeConnect.Status)
	for _, ref := range a.CodeConnect.MappingRefs {
		fmt.Fprintf(&sb, "  - mapping: %s\n", ref.Path)
	}
	for _, ref := range a.CodeConnect.PackageRefs {
		fmt.Fprintf(&sb, "  - package: %s\n", ref.Path)
	}
	if len(a.SetupGaps) > 0 {
		sb.WriteString("- Setup gaps:\n")
		for _, gap := range a.SetupGaps {
			fmt.Fprintf(&sb, "  - %s\n", gap)
		}
	}
	return sb.String()
}

func (a FigmaAudit) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func isCodeConnectMappingPath(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasSuffix(base, ".figma.ts") || strings.HasSuffix(base, ".figma.tsx") {
		return true
	}
	if strings.HasPrefix(base, "figma.config.") {
		return true
	}
	return base == "code-connect.config.json"
}

func sanitizeFigmaURL(raw string) string {
	clean, _, _, _ := parseFigmaURL(raw)
	return clean
}

func parseFigmaURL(raw string) (clean, kind, fileKey, nodeID string) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" {
		return "", "", "", ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "figma.com" && host != "www.figma.com" {
		return "", "", "", ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) > 0 {
		kind = parts[0]
	}
	if len(parts) > 1 {
		fileKey = parts[1]
	}
	nodeID = strings.ReplaceAll(parsed.Query().Get("node-id"), "-", ":")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	clean = parsed.String()
	if kind == "" {
		kind = "unknown"
	}
	return clean, kind, fileKey, nodeID
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}
