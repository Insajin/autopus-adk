package setup

import (
	"os"
	"path/filepath"
)

// CheckProviderAuth verifies whether a provider has valid credentials.
// Returns (true, "") if authenticated, or (false, guide) with instructions.
func CheckProviderAuth(name string) (authenticated bool, guide string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, "Cannot determine home directory"
	}

	switch name {
	case "claude":
		return checkClaude(home)
	case "codex":
		return checkCodex(home)
	case "gemini":
		return checkGemini(home)
	case "opencode":
		return checkOpencode()
	default:
		return false, "Unknown provider: " + name
	}
}

func checkClaude(home string) (bool, string) {
	credPath := filepath.Join(home, ".claude", "credentials.json")
	if fileExists(credPath) {
		return true, ""
	}
	return false, "1. https://claude.ai 에서 가입\n      2. npm install -g @anthropic-ai/claude-code\n      3. claude login 실행"
}

func checkCodex(home string) (bool, string) {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return true, ""
	}
	codexDir := filepath.Join(home, ".codex")
	if dirExists(codexDir) {
		return true, ""
	}
	return false, "1. https://platform.openai.com 에서 API 키 발급\n      2. 터미널에 다음 명령어를 입력하세요:\n         export OPENAI_API_KEY=여기에_키_입력\n      3. 또는 codex login 실행"
}

func checkGemini(home string) (bool, string) {
	if os.Getenv("GOOGLE_API_KEY") != "" {
		return true, ""
	}
	geminiDir := filepath.Join(home, ".config", "gemini")
	if dirExists(geminiDir) {
		return true, ""
	}
	return false, "1. https://aistudio.google.com 에서 API 키 발급\n      2. 터미널에 다음 명령어를 입력하세요:\n         export GOOGLE_API_KEY=여기에_키_입력\n      3. 또는 gemini login 실행"
}

func checkOpencode() (bool, string) {
	// opencode uses the same key as codex (OpenAI)
	if os.Getenv("OPENAI_API_KEY") != "" {
		return true, ""
	}
	return false, "1. https://platform.openai.com 에서 API 키 발급\n      2. 터미널에 다음 명령어를 입력하세요:\n         export OPENAI_API_KEY=여기에_키_입력"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
