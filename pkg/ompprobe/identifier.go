package ompprobe

// IdentifierPattern documents the metadata identifier range preserved verbatim
// by the probe (REQ-PROBE-001, INV-012). Case is significant: openaiRemoteCompaction
// and openairemotecompaction are different allowlist candidates.
const IdentifierPattern = `^[A-Za-z_][A-Za-z0-9_]{0,63}$`

const maxIdentifierLength = 64

// ValidIdentifier reports whether name is inside IdentifierPattern. It is a
// hand-rolled scan rather than a regexp so callers can use it per token on the
// hot path without allocating.
func ValidIdentifier(name string) bool {
	if len(name) == 0 || len(name) > maxIdentifierLength {
		return false
	}
	for index := range len(name) {
		character := name[index]
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z', character == '_':
		case index > 0 && character >= '0' && character <= '9':
		default:
			return false
		}
	}
	return true
}

// validReasonToken reports whether reason is a body-free lowercase diagnostic
// token. Abort reasons are operator-facing text, so they are held to a
// narrower range than preserved metadata identifiers.
func validReasonToken(reason string) bool {
	if len(reason) == 0 || len(reason) > maxIdentifierLength {
		return false
	}
	for index := range len(reason) {
		character := reason[index]
		switch {
		case character >= 'a' && character <= 'z':
		case index > 0 && (character == '_' || (character >= '0' && character <= '9')):
		default:
			return false
		}
	}
	return true
}

// Histogram counts metadata identifiers without ever aliasing an out-of-range
// token. A rejected token increments Rejected instead of colliding into a
// shared bucket, so the histogram stays usable as allowlist evidence.
type Histogram struct {
	counts   map[string]int
	rejected int
}

// Observe records one occurrence of name and reports whether it was preserved.
func (histogram *Histogram) Observe(name string) bool {
	if !ValidIdentifier(name) {
		histogram.rejected++
		return false
	}
	if histogram.counts == nil {
		histogram.counts = make(map[string]int, 8)
	}
	if len(histogram.counts) >= maxHistogramKeys {
		if _, known := histogram.counts[name]; !known {
			histogram.rejected++
			return false
		}
	}
	histogram.counts[name]++
	return true
}

// Counts returns the accumulated map, or nil when nothing was preserved. The
// zero-length case returns nil so the field stays omitted from the record.
func (histogram *Histogram) Counts() map[string]int {
	if len(histogram.counts) == 0 {
		return nil
	}
	return histogram.counts
}

// Rejected returns how many tokens fell outside IdentifierPattern or the key
// bound. Those tokens are never allowlist candidates.
func (histogram *Histogram) Rejected() int { return histogram.rejected }

// IdentifierSet collects unique identifiers in first-seen order with the same
// rejection discipline as Histogram.
type IdentifierSet struct {
	names    []string
	seen     map[string]struct{}
	rejected int
}

// Observe adds name when it is new and in range, reporting whether it was kept.
func (set *IdentifierSet) Observe(name string) bool {
	if !ValidIdentifier(name) {
		set.rejected++
		return false
	}
	if _, known := set.seen[name]; known {
		return true
	}
	if len(set.names) >= maxIdentifierList {
		set.rejected++
		return false
	}
	if set.seen == nil {
		set.seen = make(map[string]struct{}, 8)
	}
	set.seen[name] = struct{}{}
	set.names = append(set.names, name)
	return true
}

// Names returns the preserved identifiers in first-seen order, or nil when none.
func (set *IdentifierSet) Names() []string {
	if len(set.names) == 0 {
		return nil
	}
	return set.names
}

// Rejected returns how many tokens were dropped instead of preserved.
func (set *IdentifierSet) Rejected() int { return set.rejected }
