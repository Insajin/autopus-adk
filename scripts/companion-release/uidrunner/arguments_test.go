package main

import "testing"

func TestParseRunArgumentsAcceptsUIDRangeBoundaries(t *testing.T) {
	for _, test := range []struct {
		uid  string
		want int
	}{{"50000", 50000}, {"59999", 59999}} {
		t.Run(test.uid, func(t *testing.T) {
			config, err := parseRunArguments([]string{test.uid, "20", "/usr/bin/env", "true"})
			if err != nil {
				t.Fatalf("parseRunArguments() error = %v", err)
			}
			if config.uid != test.want || config.gid != 20 {
				t.Fatalf("parseRunArguments() identity = %d:%d", config.uid, config.gid)
			}
			if len(config.arguments) != 2 || config.arguments[0] != "/usr/bin/env" || config.arguments[1] != "true" {
				t.Fatalf("parseRunArguments() arguments = %#v", config.arguments)
			}
		})
	}
}

func TestParseRunArgumentsRejectsInvalidArguments(t *testing.T) {
	tests := map[string][]string{
		"missing":             nil,
		"below UID range":     {"49999", "20", "/bin/sh"},
		"above UID range":     {"60000", "20", "/bin/sh"},
		"noncanonical UID":    {"050000", "20", "/bin/sh"},
		"nonnumeric UID":      {"user", "20", "/bin/sh"},
		"wrong GID":           {"50000", "21", "/bin/sh"},
		"noncanonical GID":    {"50000", "020", "/bin/sh"},
		"empty executable":    {"50000", "20", ""},
		"relative executable": {"50000", "20", "bin/sh"},
	}
	for name, arguments := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRunArguments(arguments); err == nil {
				t.Fatal("parseRunArguments() accepted invalid arguments")
			}
		})
	}
}
