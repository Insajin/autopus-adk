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
		"@auto-", "/auto-",
		"## Codex Notes", "## OpenCode Notes",
		"## Codex 기본 실행 모델", "## OpenCode 기본 실행 모델",
		"Codex에서는", "OpenCode에서는",
		"Codex의", "OpenCode의",
		"Codex는", "OpenCode는",
		"Codex에서", "OpenCode에서",
		"spawn_agent(...)", "task(...)",
		"`spawn_agent`", "`task`",
	)
	return replacer.Replace(content)
}

func normalizeOpenCodeSkillBody(body, subcommand string) string {
	body = normalizeOpenCodeMarkdown(body)
	body = normalizeOpenCodeToolingBody(body)
	body = rewriteOpenCodeWorkflowExamples(body, subcommand)
	if subcommand == "" {
		return body
	}

	replacer := strings.NewReplacer(
		"/auto-"+subcommand, "/auto "+subcommand,
		"@auto-"+subcommand, "/auto "+subcommand,
		"$auto-"+subcommand, "/auto "+subcommand,
	)
	return replacer.Replace(body)
}

func normalizeOpenCodeToolingBody(body string) string {
	replacer := strings.NewReplacer(
		"Agent(", "task(",
		"spawn_agent(", "task(",
		"spawn_agent ", "task ",
		"AskUserQuestion(", "question(",
		"TaskCreate(", "todowrite(",
		"TaskUpdate(", "todowrite(",
		"TaskList(", "todowrite(",
		"TaskGet(", "todowrite(",
		"TeamCreate(", "task(",
		"using `spawn_agent`", "using `task`",
		"using spawn_agent", "using task",
		"`spawn_agent(...)`", "`task(...)`",
		"`spawn_agent`", "`task`",
	)
	body = replacer.Replace(body)
	body = strings.ReplaceAll(body, "task = ", "prompt = ")
	body = strings.ReplaceAll(body, "task=", "prompt=")
	body = strings.ReplaceAll(body, "Task(\n", "task(\n")
	return body
}

func rewriteOpenCodeWorkflowExamples(body, subcommand string) string {
	if subcommand != "go" {
		return body
	}

	replacer := strings.NewReplacer(
		"```\ntask executor \\\n  --task \"Implement {task description}\" \\\n  --spec \".autopus/specs/{SPEC-ID}/spec.md\" \\\n  --plan \".autopus/specs/{SPEC-ID}/plan.md\" \\\n  --constraint \"File size limit: 300 lines per source file\"\n```",
		"```text\ntask(\n  subagent_type = \"executor\",\n  prompt = \"Implement {task description}. Use .autopus/specs/{SPEC-ID}/spec.md and .autopus/specs/{SPEC-ID}/plan.md as context. Respect the 300-line file limit.\"\n)\n```",
		"```\ntask tester \\\n  --task \"Write tests for {scope}\" \\\n  --spec \".autopus/specs/{SPEC-ID}/acceptance.md\" \\\n  --coverage-target 85\n```",
		"```text\ntask(\n  subagent_type = \"tester\",\n  prompt = \"Write tests for {scope}. Use .autopus/specs/{SPEC-ID}/acceptance.md as context and target 85%% coverage.\"\n)\n```",
		"```\ntask reviewer \\\n  --task \"Review implementation for {SPEC-ID}\" \\\n  --criteria \"TRUST-5\"\n```",
		"```text\ntask(\n  subagent_type = \"reviewer\",\n  prompt = \"Review implementation for {SPEC-ID} using TRUST-5 criteria.\"\n)\n```",
	)
	return replacer.Replace(body)
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
		return "## OpenCode Arguments\n\n사용자가 `/auto` 뒤에 전달한 전체 인자는 다음과 같습니다.\n\n`$ARGUMENTS`\n\n이 command는 얇은 entrypoint입니다. 실제 라우팅 규칙은 `skill` 도구로 `auto`를 로드한 뒤 따르세요. 서브커맨드를 결정하면 대응하는 상세 스킬도 추가로 로드해야 합니다.\n"
	}
	return fmt.Sprintf("## OpenCode Arguments\n\n사용자가 `/%s` 뒤에 전달한 인자는 다음과 같습니다.\n\n`$ARGUMENTS`\n\n이 command는 얇은 entrypoint입니다. 실제 워크플로우 단계는 `skill` 도구로 `%s`를 로드한 뒤 그 스킬 문서를 기준으로 실행하세요.\n", name, name)
}

func skillInvocationNote(name string) string {
	if name == "auto" {
		return "## OpenCode Invocation\n\n이 스킬은 다음 두 경로로 사용할 수 있습니다.\n\n- `/auto <subcommand> ...`\n- `skill` 도구로 직접 `auto` 로드\n\n직접 로드되면 사용자의 최신 요청을 `/auto` 뒤 인자로 간주하고 해석하세요.\n"
	}
	subcommand := strings.TrimPrefix(name, "auto-")
	return fmt.Sprintf("## OpenCode Invocation\n\n이 스킬은 다음 두 경로로 사용할 수 있습니다.\n\n- `/auto %s ...`\n- `/%s ...`\n- `skill` 도구로 직접 `%s` 로드\n\n직접 로드되면 사용자의 최신 요청을 해당 명령의 인자로 간주하세요.\n", subcommand, name, name)
}

func injectAfterFirstHeading(body, block string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			out := make([]string, 0, len(lines)+4)
			out = append(out, lines[:i+1]...)
			out = append(out, "")
			out = append(out, block)
			out = append(out, "")
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	return block + "\n\n" + body
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

func jsonPluginSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			result = append(result, typed)
		case []any:
			if len(typed) == 0 {
				continue
			}
			if str, ok := typed[0].(string); ok {
				result = append(result, str)
			}
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toSlash(path string) string {
	return filepath.ToSlash(path)
}
