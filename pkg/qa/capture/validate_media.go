package capture

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateMedia(media []Media) error {
	if len(media) > MaxMediaEntries {
		return fmt.Errorf("media exceeds %d entries", MaxMediaEntries)
	}
	for index, entry := range media {
		switch entry.Kind {
		case StreamTrace, StreamVideo, "har", StreamScreenshot:
		default:
			return fmt.Errorf("media[%d].kind %q is unsupported", index, entry.Kind)
		}
		if err := validateMediaRef(entry.MediaRef); err != nil {
			return fmt.Errorf("media[%d]: %w", index, err)
		}
	}
	return nil
}

// validateMediaRef keeps raw capture bytes addressable but unpublishable: a
// capture-directory-relative path, a content digest, and local-only retention.
func validateMediaRef(ref MediaRef) error {
	if err := validateRelativeRef(ref.Ref); err != nil {
		return err
	}
	if !digestRe.MatchString(ref.Digest) {
		return fmt.Errorf("digest must match sha256:<64 hex>")
	}
	if ref.Bytes <= 0 {
		return fmt.Errorf("bytes must be positive")
	}
	if ref.Retention != RetentionLocalOnly {
		return fmt.Errorf("retention must be %q", RetentionLocalOnly)
	}
	return nil
}

func validateRelativeRef(value string) error {
	ref := strings.TrimSpace(value)
	if ref == "" {
		return fmt.Errorf("ref is required")
	}
	if len(ref) > MaxRefLen {
		return fmt.Errorf("ref exceeds %d characters", MaxRefLen)
	}
	if strings.ContainsAny(ref, "\x00\r\n\t") {
		return fmt.Errorf("ref must not contain control characters")
	}
	if filepath.IsAbs(ref) || strings.Contains(ref, ":\\") || strings.HasPrefix(ref, "\\") {
		return fmt.Errorf("ref must be relative to the capture directory")
	}
	clean := filepath.ToSlash(filepath.Clean(ref))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("ref must not escape the capture directory")
	}
	return nil
}

func validateReplay(replay *Replay) error {
	if replay == nil {
		return nil
	}
	if replay.Kind != ReplayKindPlaywrightGrep {
		return fmt.Errorf("replay.kind %q is unsupported", replay.Kind)
	}
	if len(replay.Command) == 0 {
		return fmt.Errorf("replay.command is required")
	}
	if len(replay.Command) > MaxReplayCommandLen {
		return fmt.Errorf("replay.command exceeds %d arguments", MaxReplayCommandLen)
	}
	for index, arg := range replay.Command {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("replay.command[%d] is empty", index)
		}
		if shellMetaRe.MatchString(arg) {
			return fmt.Errorf("replay.command[%d] must not contain shell metacharacters", index)
		}
	}
	if len(replay.SpecRefs) > MaxSpecRefs {
		return fmt.Errorf("replay.spec_refs exceed %d entries", MaxSpecRefs)
	}
	for index, ref := range replay.SpecRefs {
		if err := validateRelativeRef(ref); err != nil {
			return fmt.Errorf("replay.spec_refs[%d]: %w", index, err)
		}
	}
	if !digestRe.MatchString(replay.Digest) {
		return fmt.Errorf("replay.digest must match sha256:<64 hex>")
	}
	return nil
}
