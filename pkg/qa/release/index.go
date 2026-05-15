package release

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func writeIndex(index Index, path string) error {
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
