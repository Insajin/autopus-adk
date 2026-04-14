package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func splitFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content
	}
	frontmatter := rest[:idx]
	body := rest[idx+4:]
	body = strings.TrimPrefix(body, "\n")
	return frontmatter, body
}

func buildMarkdown(frontmatter, body string) string {
	if strings.TrimSpace(frontmatter) == "" {
		return strings.TrimSpace(body) + "\n"
	}
	return fmt.Sprintf("---\n%s\n---\n\n%s\n", strings.TrimSpace(frontmatter), strings.TrimSpace(body))
}

func normalizeOpenCodeMarkdown(content string) string {
	replacer := strings.NewReplacer(
		"@auto ", "/auto ",
		"## Codex Notes", "## OpenCode Notes",
		"Codex에서는", "OpenCode에서는",
		"Codex의", "OpenCode의",
		"Codex는", "OpenCode는",
		"Codex에서", "OpenCode에서",
		"spawn_agent(...)", "task(...)",
		"`spawn_agent`", "`task`",
	)
	return replacer.Replace(content)
}

func augmentCommandFrontmatter(frontmatter string) string {
	frontmatter = strings.TrimSpace(frontmatter)
	if frontmatter == "" {
		return "description: \"Autopus command\"\nagent: build"
	}
	if strings.Contains(frontmatter, "\nagent:") || strings.HasPrefix(frontmatter, "agent:") {
		return frontmatter
	}
	return frontmatter + "\nagent: build"
}

func commandArgumentNote(name string) string {
	if name == "auto" {
		return "## OpenCode Arguments\n\n사용자가 `/auto` 뒤에 전달한 전체 인자는 다음과 같습니다.\n\n`$ARGUMENTS`\n"
	}
	return fmt.Sprintf("## OpenCode Arguments\n\n사용자가 `/%s` 뒤에 전달한 인자는 다음과 같습니다.\n\n`$ARGUMENTS`\n", name)
}

func skillInvocationNote(name string) string {
	if name == "auto" {
		return "## OpenCode Invocation\n\n이 스킬은 다음 두 경로로 사용할 수 있습니다.\n\n- `/auto <subcommand> ...`\n- `skill` 도구로 직접 `auto` 로드\n\n직접 로드되면 사용자의 최신 요청을 `/auto` 뒤 인자로 간주하고 해석하세요.\n"
	}
	subcommand := strings.TrimPrefix(name, "auto-")
	return fmt.Sprintf("## OpenCode Invocation\n\n이 스킬은 다음 두 경로로 사용할 수 있습니다.\n\n- `/auto %s ...`\n- `/%s ...`\n- `skill` 도구로 직접 `%s` 로드\n\n직접 로드되면 사용자의 최신 요청을 해당 명령의 인자로 간주하세요.\n", subcommand, name, name)
}

func uniqueStrings(values ...[]string) []string {
	seen := map[string]bool{}
	var result []string
	for _, list := range values {
		for _, item := range list {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func jsonStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func toSlash(path string) string {
	return filepath.ToSlash(path)
}
