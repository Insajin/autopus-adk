package workflow

import (
	"strconv"
	"strings"
)

// versionAtLeast reports whether the dotted-int version `got` is greater than or
// equal to `min`. Non-numeric or empty segments are treated as 0 so a missing
// or unparseable version compares below any real pin (e.g. "" < "2.1.154").
func versionAtLeast(got, min string) bool {
	gotParts := parseVersion(got)
	minParts := parseVersion(min)

	n := len(gotParts)
	if len(minParts) > n {
		n = len(minParts)
	}
	for i := 0; i < n; i++ {
		g := versionSegment(gotParts, i)
		m := versionSegment(minParts, i)
		if g != m {
			return g > m
		}
	}
	return true
}

func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	fields := strings.Split(v, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

func versionSegment(parts []int, i int) int {
	if i < len(parts) {
		return parts[i]
	}
	return 0
}
