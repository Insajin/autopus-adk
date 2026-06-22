//go:build !unix

package workflow

import "os"

// hardLinked cannot be determined portably off Unix; treat as not hard-linked.
func hardLinked(os.FileInfo) bool { return false }

// openNoFollow falls back to a plain open where O_NOFOLLOW is unavailable.
func openNoFollow(src string) (*os.File, error) { return os.Open(src) }
