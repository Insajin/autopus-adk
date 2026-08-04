package promptlayer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var ompWindowsAbsolutePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func validateOMPContextMetadata(field, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 256 {
		return "", fmt.Errorf("OMP context %s is missing or too long", field)
	}
	if filepath.IsAbs(value) || ompWindowsAbsolutePath.MatchString(value) || strings.HasPrefix(value, `\\`) {
		return "", fmt.Errorf("OMP context %s must not be an absolute path", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("OMP context %s contains control characters", field)
		}
	}
	for _, pattern := range contextSecretPatterns {
		if pattern.MatchString(value) {
			return "", fmt.Errorf("OMP context %s contains secret-like metadata", field)
		}
	}
	if hasInjectionMarker(value) {
		return "", fmt.Errorf("OMP context %s contains prompt-injection metadata", field)
	}
	return value, nil
}

func normalizeOMPContextMetadataList(field string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		value, err := validateOMPContextMetadata(field, raw)
		if err != nil {
			return nil, err
		}
		if seen[value] {
			return nil, fmt.Errorf("OMP context %s contains duplicate value: %s", field, value)
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, nil
}

func normalizeOMPContextPaths(field string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, raw := range values {
		if _, err := validateOMPContextMetadata(field, raw); err != nil {
			return nil, err
		}
		value, err := cleanContextReference(raw, true)
		if err != nil {
			return nil, fmt.Errorf("invalid OMP context %s: %w", field, err)
		}
		if seen[value] {
			return nil, fmt.Errorf("OMP context %s contains duplicate path: %s", field, value)
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func ompContextOptionsHash(opts ContextDeliveryOptions) (string, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	command := strings.ToLower(strings.TrimSpace(opts.Command))
	profile, ok := ResolveCommandContextProfile(command)
	if !ok {
		return "", fmt.Errorf("unknown context profile command: %s", command)
	}
	specDir, err := cleanContextReference(opts.SpecDir, profile.RelevantSpec)
	if err != nil {
		return "", fmt.Errorf("invalid spec directory: %w", err)
	}
	refs, err := cleanUniqueContextReferences(opts.RequiredReferences)
	if err != nil {
		return "", err
	}
	sort.Strings(refs)
	conditional, err := resolveConditionalContextProfiles(command, profile, opts.ConditionalProfiles)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve OMP context root: %w", err)
	}
	material, err := json.Marshal(struct {
		RootHash     string               `json:"root_hash"`
		Command      string               `json:"command"`
		SpecDir      string               `json:"spec_dir"`
		RequiredRefs []string             `json:"required_refs"`
		Conditional  []ContextProfileName `json:"conditional_profiles"`
	}{
		RootHash: canonicalHash([]byte(filepath.Clean(absRoot))), Command: command,
		SpecDir: specDir, RequiredRefs: refs, Conditional: conditional,
	})
	if err != nil {
		return "", fmt.Errorf("encode OMP context options: %w", err)
	}
	return canonicalHash(material), nil
}

func isOMPContextHash(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, r := range value[len("sha256:"):] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func mustOMPContextJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
