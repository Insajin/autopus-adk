package config

import "slices"

func migrateAntigravityGeminiProvider(existing, defaults ProviderEntry, replaceEmptyArgs, changed bool) (ProviderEntry, bool) {
	if existing.Binary == "" || existing.Binary == "gemini" {
		existing.Binary = defaults.Binary
		changed = true
	}
	if isLegacyGeminiCLIArgs(existing.Args) || (replaceEmptyArgs && len(existing.Args) == 0) {
		existing.Args = append([]string{}, defaults.Args...)
		changed = true
	}
	return existing, changed
}

func isLegacyGeminiCLIArgs(args []string) bool {
	return slices.Equal(args, []string{"-m", "gemini-3.1-pro-preview", "-p", ""})
}
