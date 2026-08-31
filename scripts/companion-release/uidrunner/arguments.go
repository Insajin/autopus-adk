package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
)

const (
	releaseCanaryUIDMin = 50000
	releaseCanaryUIDMax = 59999
	releaseCanaryGID    = 20
)

type runConfig struct {
	uid       int
	gid       int
	arguments []string
}

func parseRunArguments(arguments []string) (runConfig, error) {
	if len(arguments) < 3 {
		return runConfig{}, errors.New("UID, GID, and executable are required")
	}
	uid, err := parseCanonicalID(arguments[0])
	if err != nil || uid < releaseCanaryUIDMin || uid > releaseCanaryUIDMax {
		return runConfig{}, fmt.Errorf("UID must be between %d and %d", releaseCanaryUIDMin, releaseCanaryUIDMax)
	}
	gid, err := parseCanonicalID(arguments[1])
	if err != nil || gid != releaseCanaryGID {
		return runConfig{}, fmt.Errorf("GID must be %d", releaseCanaryGID)
	}
	if arguments[2] == "" || !filepath.IsAbs(arguments[2]) {
		return runConfig{}, errors.New("executable must be a nonempty absolute path")
	}
	return runConfig{uid: uid, gid: gid, arguments: arguments[2:]}, nil
}

func parseCanonicalID(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || strconv.Itoa(parsed) != value {
		return 0, errors.New("identity is not canonical decimal")
	}
	return parsed, nil
}
