package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

var forbiddenDesktopPublicationText = regexp.MustCompile(
	`(?i)(raw[_ -]?ax|raw_tree|screen[_ -]?shot|\.(?:png|jpe?g|gif|webp|tiff?|bmp|heic|mov|mp4)\b|` +
		`(?:file://)?/(?:Users|home)/|/tmp/|/private/(?:tmp|var)/|/var/folders/|/Volumes/|/Applications/|` +
		`[A-Za-z]:\\+Users\\+|0x[0-9a-f]{4,})`,
)

var forbiddenDesktopPublicationKeys = map[string]struct{}{
	"binarypath":      {},
	"errortext":       {},
	"handle":          {},
	"helperpath":      {},
	"index":           {},
	"pid":             {},
	"processid":       {},
	"providerpayload": {},
	"rawax":           {},
	"rawhandle":       {},
	"rawindex":        {},
	"rawpayload":      {},
	"rawstderr":       {},
	"rawstdout":       {},
	"rawtree":         {},
	"screenshot":      {},
	"screenshots":     {},
	"socket":          {},
	"stderr":          {},
	"stdout":          {},
}

func scanDesktopObservationPublication(root string, manifest Manifest) error {
	expected := map[string]struct{}{
		"manifest.json": {},
		filepath.ToSlash(manifest.Artifacts[0].Path): {},
	}
	seen := make(map[string]struct{}, len(expected))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		_, allowed := expected[relative]
		if walkErr != nil || entry == nil ||
			(!entry.IsDir() && (entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !allowed)) {
			return fmt.Errorf("desktop observation publication inventory contains an unsafe output")
		}
		if entry.IsDir() {
			return nil
		}
		seen[relative] = struct{}{}
		return scanDesktopObservationFile(path, relative)
	})
	if err != nil || len(seen) != len(expected) {
		return fmt.Errorf("desktop observation publication inventory is incomplete")
	}
	return nil
}

func scanDesktopObservationFile(path, source string) error {
	body, err := os.ReadFile(path)
	text := string(body)
	var value any
	parseErr := json.Unmarshal(body, &value)
	if err != nil || parseErr != nil || RedactText(text) != text || AssertSafeText(text, source) != nil ||
		forbiddenDesktopPublicationText.MatchString(text) || containsForbiddenDesktopObservationValue(value) {
		return fmt.Errorf("desktop observation publication contains forbidden raw, local, or malformed data")
	}
	return nil
}

func containsForbiddenDesktopObservationValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		found := false
		for key, child := range typed {
			normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			_, forbidden := forbiddenDesktopPublicationKeys[normalized]
			found = found || forbidden || containsForbiddenDesktopObservationValue(child)
		}
		return found
	case []any:
		found := false
		for _, child := range typed {
			found = found || containsForbiddenDesktopObservationValue(child)
		}
		return found
	case string:
		return forbiddenDesktopPublicationText.MatchString(typed)
	}
	return false
}

func rejectManifestDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := consumeUniqueManifestValue(decoder); err != nil {
		return err
	}
	return requireManifestJSONEOF(decoder)
}

func consumeUniqueManifestValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %v", desktopobserve.ErrMalformedEnvelope, err)
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	if delimiter == '[' {
		for decoder.More() {
			if err := consumeUniqueManifestValue(decoder); err != nil {
				return err
			}
		}
		return consumeManifestDelimiter(decoder, ']')
	}
	if delimiter != '{' {
		return desktopobserve.ErrMalformedEnvelope
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", desktopobserve.ErrMalformedEnvelope, err)
		}
		name, ok := key.(string)
		if !ok {
			return desktopobserve.ErrMalformedEnvelope
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("%w: %s", desktopobserve.ErrDuplicateKey, name)
		}
		seen[name] = struct{}{}
		if err := consumeUniqueManifestValue(decoder); err != nil {
			return err
		}
	}
	return consumeManifestDelimiter(decoder, '}')
}

func consumeManifestDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return desktopobserve.ErrMalformedEnvelope
	}
	return nil
}

func requireManifestJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		return desktopobserve.ErrMalformedEnvelope
	}
	return nil
}
