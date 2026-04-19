package orchestra

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"
)

// waitForSentinel polls the output file until the sentinel marker is found.
func waitForSentinel(ctx context.Context, outputFile string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if hasSentinel(outputFile) {
				return nil
			}
		}
	}
}

// hasSentinel checks if the output file contains the sentinel marker.
func hasSentinel(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), sentinel) {
			return true
		}
	}
	return false
}

// readOutputFile reads the output file and strips the sentinel marker.
func readOutputFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	output := strings.ReplaceAll(string(data), sentinel, "")
	return strings.TrimSpace(output)
}
