package gates

import (
	"path"
	"strings"
)

// This file holds the path predicates the declared-class rules read: which
// paths are test material, which are a published contract surface, and which
// production module roots a change set spans. Keeping them next to each other
// means the escalation rules and the surface definitions cannot drift apart.

// declaredSurfaceViolation names the escalation reason when the change set
// contains a path the declared class does not cover, or "" when every path
// belongs to the declared surface. Documentation never contradicts a class.
func declaredSurfaceViolation(declared ChangeKind, c Classification) string {
	ui := uiPathSet(c)
	for _, p := range c.Paths {
		if IsDocPath(p) {
			continue
		}
		switch declared {
		case KindTestOnly:
			if !IsTestPath(p) {
				return EscalationTestOnlyCode
			}
		case KindDocsOnly:
			return EscalationDocsOnlyCode
		case KindSmallUI:
			if !IsTestPath(p) && !ui[p] {
				return EscalationSmallUINonUI
			}
		}
	}
	return ""
}

// hasNonTestCode reports whether the change set carries code that is neither
// documentation nor test material.
func hasNonTestCode(c Classification) bool {
	for _, p := range c.Paths {
		if !IsDocPath(p) && !IsTestPath(p) {
			return true
		}
	}
	return false
}

// productionRoots returns the distinct module roots spanned by production
// code: paths that are neither documentation nor test material.
func productionRoots(c Classification) []string {
	seen := map[string]bool{}
	var roots []string
	for _, p := range c.Paths {
		if IsDocPath(p) || IsTestPath(p) {
			continue
		}
		root := moduleRoot(p)
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		roots = append(roots, root)
	}
	return roots
}

func uiPathSet(c Classification) map[string]bool {
	set := make(map[string]bool, len(c.UIPaths))
	for _, p := range c.UIPaths {
		set[p] = true
	}
	return set
}

func contractPaths(c Classification) []string {
	var out []string
	for _, p := range c.Paths {
		if IsContractPath(p) {
			out = append(out, p)
		}
	}
	return out
}

var testDirSegments = map[string]bool{
	"test": true, "tests": true, "testdata": true, "__tests__": true,
}

var testStemSuffixes = []string{"_test", ".test", "_spec", ".spec"}

// IsTestPath reports whether p is test code, test data, or a test fixture.
// Directory names decide as well as file names, so a fixture without a test
// suffix still counts.
func IsTestPath(p string) bool {
	p = normalizePath(p)
	if p == "" {
		return false
	}
	base := strings.ToLower(path.Base(p))
	stem := strings.TrimSuffix(base, path.Ext(base))
	if strings.HasPrefix(stem, "test_") {
		return true
	}
	for _, suffix := range testStemSuffixes {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	for _, segment := range strings.Split(path.Dir(p), "/") {
		if testDirSegments[strings.ToLower(segment)] {
			return true
		}
	}
	return false
}

var contractRootPrefixes = []string{
	"api/", "proto/", "openapi/", "swagger/", "graphql/",
	"internal/api/", "pkg/api/", "src/api/",
}

var contractExtensions = map[string]bool{
	".proto": true, ".graphql": true, ".gql": true,
}

var contractBasePrefixes = []string{"openapi.", "swagger."}

// IsContractPath reports whether p is a published interface surface: an IDL
// file or a path under a public API root. Editing one is a contract change
// even when the declared class is small.
func IsContractPath(p string) bool {
	lower := strings.ToLower(normalizePath(p))
	if lower == "" {
		return false
	}
	if contractExtensions[path.Ext(lower)] {
		return true
	}
	base := path.Base(lower)
	for _, prefix := range contractBasePrefixes {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	for _, prefix := range contractRootPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func normalizePath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" {
		return ""
	}
	p = path.Clean(p)
	if p == "." || p == "/" {
		return ""
	}
	return strings.TrimPrefix(p, "./")
}
