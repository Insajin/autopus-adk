package detect

// DetectInstalledPlatforms returns supported coding CLIs found in PATH without
// executing their version commands. Results preserve knownCLIs order.
func DetectInstalledPlatforms() []Platform {
	platforms := make([]Platform, 0, len(knownCLIs))
	for _, cli := range knownCLIs {
		if !IsInstalled(cli.binary) {
			continue
		}
		platforms = append(platforms, Platform{Name: cli.name, Binary: cli.binary})
	}
	return platforms
}
