package memindex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/learn"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

func Scan(projectDir string) ([]Record, []Skip, error) {
	if projectDir == "" {
		projectDir = "."
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, nil, err
	}
	records := make([]Record, 0)
	skips := generatedSurfaceSkips(abs)
	add := func(next []Record, nextSkips []Skip) {
		records = append(records, next...)
		skips = append(skips, nextSkips...)
	}
	docs, docSkips, err := scanMarkdownSources(abs)
	if err != nil {
		return nil, nil, err
	}
	add(docs, docSkips)
	learning, learnSkips, err := scanLearning(abs)
	if err != nil {
		return nil, nil, err
	}
	add(learning, learnSkips)
	qamesh, qameshSkips, err := scanQAMESH(abs)
	if err != nil {
		return nil, nil, err
	}
	add(qamesh, qameshSkips)
	qualityLoop, qualityLoopSkips, err := scanImprovementCandidates(abs)
	if err != nil {
		return nil, nil, err
	}
	add(qualityLoop, qualityLoopSkips)
	sort.Slice(records, func(i, j int) bool {
		if records[i].SourceType != records[j].SourceType {
			return records[i].SourceType < records[j].SourceType
		}
		return records[i].SourceRef < records[j].SourceRef
	})
	sort.Slice(skips, func(i, j int) bool {
		if skips[i].Reason != skips[j].Reason {
			return skips[i].Reason < skips[j].Reason
		}
		return skips[i].Path < skips[j].Path
	})
	return records, skips, nil
}

func scanMarkdownSources(projectDir string) ([]Record, []Skip, error) {
	roots := []string{
		filepath.Join(projectDir, ".autopus", "project"),
		filepath.Join(projectDir, ".autopus", "specs"),
	}
	records := make([]Record, 0)
	skips := make([]Skip, 0)
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := slashRel(projectDir, path)
			if entry.IsDir() {
				if deniedGeneratedSurface(rel) {
					skips = append(skips, Skip{Path: rel, Reason: "generated_surface"})
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".md" {
				return nil
			}
			record, ok, skip, err := markdownRecord(projectDir, path)
			if err != nil {
				return err
			}
			if !ok {
				skips = append(skips, skip)
				return nil
			}
			records = append(records, record)
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return records, skips, nil
}

func markdownRecord(projectDir, path string) (Record, bool, Skip, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Record{}, false, Skip{}, err
	}
	rel := slashRel(projectDir, path)
	if findings := qaevidence.FindUnsafeText(string(body), rel); len(findings) > 0 {
		return Record{}, false, Skip{Path: rel, Reason: "unsafe_source_text"}, nil
	}
	hash, err := hashFile(path)
	if err != nil {
		return Record{}, false, Skip{}, err
	}
	bodyText := string(body)
	return Record{
		SourceType:      sourceKindFromPath(rel),
		SourceRef:       rel,
		SourceHash:      hash,
		Title:           safeText(titleFromMarkdown(path, body)),
		Summary:         summaryFromMarkdown(body),
		Tags:            tagsFromPath(rel),
		SpecID:          detectSpecID(rel, bodyText),
		AcceptanceIDs:   acceptanceIDs(bodyText),
		Timestamp:       fileTimestamp(path),
		RedactionStatus: Redacted,
		Content:         safeText(bodyText),
	}, true, Skip{}, nil
}

func scanLearning(projectDir string) ([]Record, []Skip, error) {
	path := filepath.Join(projectDir, ".autopus", "learnings", "pipeline.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	defer f.Close()
	records := make([]Record, 0)
	skips := make([]Skip, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry learn.LearningEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			skips = append(skips, Skip{Path: ".autopus/learnings/pipeline.jsonl", Reason: "invalid_learning_jsonl"})
			continue
		}
		rel := ".autopus/learnings/pipeline.jsonl#" + entry.ID
		metadata := learningMetadata(entry)
		text := learningSearchText(entry, metadata)
		if findings := qaevidence.FindUnsafeText(text, rel); len(findings) > 0 {
			skips = append(skips, Skip{Path: rel, Reason: "unsafe_source_text"})
			continue
		}
		records = append(records, Record{
			SourceType:      "learning",
			SourceRef:       entry.ID,
			SourceHash:      hashBytes([]byte(line)),
			Title:           safeText(entry.Pattern),
			Summary:         safeText(entry.Resolution),
			Tags:            []string{string(entry.Type), entry.Phase},
			SpecID:          entry.SpecID,
			FileRefs:        entry.Files,
			PackageRefs:     entry.Packages,
			Severity:        string(entry.Severity),
			Timestamp:       entry.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			RedactionStatus: Redacted,
			Content:         safeText(text),
			SourceMetadata:  metadata,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	return records, skips, nil
}

func learningMetadata(entry learn.LearningEntry) map[string]any {
	return map[string]any{
		"type":        string(entry.Type),
		"phase":       entry.Phase,
		"spec_id":     entry.SpecID,
		"files":       entry.Files,
		"packages":    entry.Packages,
		"pattern":     entry.Pattern,
		"resolution":  entry.Resolution,
		"severity":    string(entry.Severity),
		"reuse_count": entry.ReuseCount,
	}
}

func learningSearchText(entry learn.LearningEntry, metadata map[string]any) string {
	body, _ := json.Marshal(metadata)
	return fmt.Sprintf("%s %s %s %s %s %s", entry.Type, entry.Phase, entry.Pattern, entry.Resolution, entry.Severity, string(body))
}

func generatedSurfaceSkips(projectDir string) []Skip {
	candidates := []string{
		".codex",
		".claude",
		".gemini",
		".opencode",
		filepath.ToSlash(filepath.Join(".autopus", "plugins")),
		filepath.ToSlash(filepath.Join(".autopus", "orchestra")),
		filepath.ToSlash(filepath.Join(".autopus", "brainstorms")),
	}
	skips := make([]Skip, 0)
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel))); err == nil {
			skips = append(skips, Skip{Path: rel, Reason: "generated_surface"})
		}
	}
	return skips
}

func deniedGeneratedSurface(rel string) bool {
	rel = strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	return rel == ".codex" || strings.HasPrefix(rel, ".codex/") ||
		rel == ".claude" || strings.HasPrefix(rel, ".claude/") ||
		rel == ".gemini" || strings.HasPrefix(rel, ".gemini/") ||
		rel == ".opencode" || strings.HasPrefix(rel, ".opencode/") ||
		rel == ".autopus/plugins" || strings.HasPrefix(rel, ".autopus/plugins/") ||
		rel == ".autopus/orchestra" || strings.HasPrefix(rel, ".autopus/orchestra/") ||
		rel == ".autopus/brainstorms" || strings.HasPrefix(rel, ".autopus/brainstorms/")
}

func acceptanceIDs(body string) []string {
	re := regexpAcceptanceID()
	matches := re.FindAllString(body, -1)
	sort.Strings(matches)
	return uniqueStrings(matches)
}
