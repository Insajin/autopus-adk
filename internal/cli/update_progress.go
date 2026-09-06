package cli

import (
	"fmt"
	"io"
	"strings"
)

func updatePlatformLabel(platform string) string {
	if platform == "omp" {
		return "OMP (Oh My Pi)"
	}
	return platform
}

func reportUpdateTargets(out io.Writer, configured, detected []string) {
	labels := make([]string, 0, len(configured)+len(detected))
	for _, platform := range configured {
		labels = append(labels, updatePlatformLabel(platform))
	}
	for _, platform := range detected {
		labels = append(labels, updatePlatformLabel(platform))
	}
	fmt.Fprintf(out, "Update targets (%d): %s\n", len(labels), strings.Join(labels, ", "))
}
