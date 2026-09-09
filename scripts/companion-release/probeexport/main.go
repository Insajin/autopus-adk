// Command probeexport republishes an OMP context probe record file out of the
// disposable canary workspace into a retained directory (SPEC-OMP-007
// REQ-PROBE-001).
//
// The release lane runs it only after the canary UID process set has been
// confirmed absent. It never copies raw bytes: every record is decoded and
// validated in memory, and only re-serialized records are written to a newly
// created runner-owned 0600 file.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

func main() {
	summary, err := run(os.Args[1:])
	// The summary is always printed so the release lane can parse counts on
	// both paths; the reason is a fixed string and never quotes record bytes.
	if encoded, encodeErr := json.Marshal(summary); encodeErr == nil {
		fmt.Println(string(encoded))
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "probe export: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) (ompprobe.ExportSummary, error) {
	flags := flag.NewFlagSet("probeexport", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sourceRoot := flags.String("source-root", "", "absolute canary workspace root holding the probe records")
	sourceRelative := flags.String("source-relative", "", "record file path relative to the source root")
	destinationRoot := flags.String("destination-root", "", "absolute retained directory that receives the records")
	destinationName := flags.String("destination-name", "", "retained file name, probe-<nonce>.jsonl")
	sourceUID := flags.String("source-uid", "", "UID that must own the source record file")
	ownerUID := flags.String("owner-uid", "", "UID that must own the retained file")
	ownerGID := flags.String("owner-gid", "", "GID that must own the retained file")
	if err := flags.Parse(arguments); err != nil {
		return ompprobe.ExportSummary{}, errors.New("arguments are invalid")
	}
	if flags.NArg() != 0 {
		return ompprobe.ExportSummary{}, errors.New("positional arguments are not accepted")
	}
	options := ompprobe.ExportOptions{
		SourceRoot:      *sourceRoot,
		SourceRelative:  *sourceRelative,
		DestinationRoot: *destinationRoot,
		DestinationName: *destinationName,
	}
	identities := []struct {
		name   string
		value  string
		target *uint32
	}{
		{name: "source-uid", value: *sourceUID, target: &options.SourceUID},
		{name: "owner-uid", value: *ownerUID, target: &options.OwnerUID},
		{name: "owner-gid", value: *ownerGID, target: &options.OwnerGID},
	}
	for _, identity := range identities {
		parsed, err := parseIdentity(identity.value)
		if err != nil {
			return ompprobe.ExportSummary{}, fmt.Errorf("%s is invalid", identity.name)
		}
		*identity.target = parsed
	}
	return ompprobe.Export(options)
}

// parseIdentity accepts only canonical decimal so a padded or signed argument
// cannot silently resolve to a different account than the lane intended.
func parseIdentity(value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return 0, errors.New("identity is not canonical decimal")
	}
	return uint32(parsed), nil
}
