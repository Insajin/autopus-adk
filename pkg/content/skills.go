// Package content는 스킬, 에이전트, 방법론 등 콘텐츠 정의 및 변환을 담당한다.
package content

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillDefinition은 3-tier Tiered Loading 스킬 정의이다.
type SkillDefinition struct {
	// Name은 스킬 이름이다.
	Name string `yaml:"name"`
	// Description은 스킬 설명이다.
	Description string `yaml:"description"`
	// Level1Metadata는 ~100 토큰 메타데이터이다 (항상 로드).
	Level1Metadata string `yaml:"level1_metadata"`
	// Level2Body는 ~5K 본문이다 (트리거 매칭 시 로드).
	Level2Body string `yaml:"-"`
	// Level3Resources는 온디맨드 리소스 목록이다.
	Level3Resources []string `yaml:"level3_resources"`
	// Triggers는 스킬 활성화 트리거 패턴이다.
	Triggers []string `yaml:"triggers"`
	// Category는 스킬 카테고리이다.
	Category string `yaml:"category"`
}

// skillFrontmatter는 마크다운 프론트매터 파싱용 내부 구조체이다.
type skillFrontmatter struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Level1Metadata  string   `yaml:"level1_metadata"`
	Level3Resources []string `yaml:"level3_resources"`
	Triggers        []string `yaml:"triggers"`
	Category        string   `yaml:"category"`
}

// SkillRegistry는 스킬 레지스트리이다.
type SkillRegistry struct {
	skills map[string]SkillDefinition
}

// Load는 디렉토리에서 스킬 파일을 로드한다.
func (r *SkillRegistry) Load(dir string) error {
	r.skills = make(map[string]SkillDefinition)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("스킬 디렉토리 읽기 실패 %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		skill, err := parseSkillFile(path)
		if err != nil {
			return fmt.Errorf("스킬 파일 파싱 실패 %s: %w", path, err)
		}
		r.skills[skill.Name] = skill
	}
	return nil
}

// Get은 이름으로 스킬을 반환한다.
func (r *SkillRegistry) Get(name string) (SkillDefinition, error) {
	skill, ok := r.skills[name]
	if !ok {
		return SkillDefinition{}, fmt.Errorf("스킬 %q을 찾을 수 없음", name)
	}
	return skill, nil
}

// List는 모든 스킬을 반환한다.
func (r *SkillRegistry) List() []SkillDefinition {
	result := make([]SkillDefinition, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// ListByCategory는 카테고리별 스킬을 반환한다.
func (r *SkillRegistry) ListByCategory(cat string) []SkillDefinition {
	var result []SkillDefinition
	for _, s := range r.skills {
		if s.Category == cat {
			result = append(result, s)
		}
	}
	return result
}

// ConvertSkillToPlatform은 스킬을 플랫폼 형식으로 변환한다.
// claude: .claude/skills/autopus/<name>.md
// codex: .codex/skills/auto-<name>/SKILL.md
// gemini: .gemini/skills/auto-<name>/SKILL.md (YAML frontmatter 포함)
func ConvertSkillToPlatform(skill SkillDefinition, platform string) (string, error) {
	switch platform {
	case "claude", "claude-code":
		return convertSkillClaude(skill), nil
	case "codex":
		return convertSkillCodex(skill), nil
	case "gemini", "gemini-cli":
		return convertSkillGemini(skill), nil
	default:
		return "", fmt.Errorf("지원하지 않는 플랫폼: %q", platform)
	}
}

// convertSkillClaude는 Claude 형식으로 변환한다.
func convertSkillClaude(skill SkillDefinition) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", skill.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", skill.Description))
	if len(skill.Triggers) > 0 {
		sb.WriteString("triggers:\n")
		for _, t := range skill.Triggers {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}
	sb.WriteString("---\n\n")
	if skill.Level2Body != "" {
		sb.WriteString(skill.Level2Body)
	}
	return sb.String()
}

// convertSkillCodex는 Codex 형식으로 변환한다.
func convertSkillCodex(skill SkillDefinition) string {
	var sb strings.Builder
	// codex는 auto-<name> 접두사 사용
	sb.WriteString(fmt.Sprintf("# auto-%s\n\n", skill.Name))
	sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", skill.Description))
	if len(skill.Triggers) > 0 {
		sb.WriteString("**Triggers:** ")
		sb.WriteString(strings.Join(skill.Triggers, ", "))
		sb.WriteString("\n\n")
	}
	if skill.Level2Body != "" {
		sb.WriteString(skill.Level2Body)
	}
	return sb.String()
}

// convertSkillGemini는 Gemini 형식으로 변환한다 (YAML frontmatter 포함).
func convertSkillGemini(skill SkillDefinition) string {
	var sb strings.Builder
	// gemini는 YAML frontmatter 필수
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: auto-%s\n", skill.Name))
	sb.WriteString(fmt.Sprintf("description: %s\n", skill.Description))
	if len(skill.Triggers) > 0 {
		sb.WriteString("triggers:\n")
		for _, t := range skill.Triggers {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
	}
	sb.WriteString("---\n\n")
	if skill.Level2Body != "" {
		sb.WriteString(skill.Level2Body)
	}
	return sb.String()
}

// LoadFromFS loads skill files from an embedded filesystem.
func (r *SkillRegistry) LoadFromFS(fsys fs.FS, dir string) error {
	r.skills = make(map[string]SkillDefinition)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("스킬 디렉토리 읽기 실패 %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := fs.ReadFile(fsys, dir+"/"+entry.Name())
		if err != nil {
			return fmt.Errorf("스킬 파일 읽기 실패 %s: %w", entry.Name(), err)
		}

		skill, err := parseSkillData(data, entry.Name())
		if err != nil {
			return fmt.Errorf("스킬 파일 파싱 실패 %s: %w", entry.Name(), err)
		}
		r.skills[skill.Name] = skill
	}
	return nil
}

// parseSkillFile은 마크다운 스킬 파일을 파싱한다.
func parseSkillFile(path string) (SkillDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillDefinition{}, fmt.Errorf("파일 읽기 실패: %w", err)
	}
	return parseSkillData(data, filepath.Base(path))
}

// parseSkillData parses skill definition from raw bytes.
func parseSkillData(data []byte, filename string) (SkillDefinition, error) {
	raw := string(data)
	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		return SkillDefinition{}, fmt.Errorf("프론트매터 파싱 실패: %w", err)
	}

	var frontmatter skillFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &frontmatter); err != nil {
		return SkillDefinition{}, fmt.Errorf("YAML 파싱 실패: %w", err)
	}

	skill := SkillDefinition{
		Name:            frontmatter.Name,
		Description:     frontmatter.Description,
		Level1Metadata:  frontmatter.Level1Metadata,
		Level2Body:      strings.TrimSpace(body),
		Level3Resources: frontmatter.Level3Resources,
		Triggers:        frontmatter.Triggers,
		Category:        frontmatter.Category,
	}

	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filename, ".md")
	}

	return skill, nil
}

// splitFrontmatter는 마크다운 컨텐츠를 프론트매터와 본문으로 분리한다.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	// 첫 번째 --- 이후 두 번째 --- 찾기
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content, nil
	}

	frontmatter = rest[:idx]
	body = rest[idx+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return frontmatter, body, nil
}
