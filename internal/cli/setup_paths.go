package cli

func resolveDirFromArgs(args []string) (string, error) {
	if len(args) > 0 { //nolint:revive
		return resolveDir(args[0])
	}
	return resolveDir("")
}

func resolveOutputDir(projectDir, outputDir string) string {
	if outputDir != "" { //nolint:revive
		return outputDir
	}
	return projectDir + "/.autopus/docs"
}
