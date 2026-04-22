package config

import "strings"

type StatusLineMode string

const (
	StatusLineModeKeep    StatusLineMode = "keep"
	StatusLineModeMerge   StatusLineMode = "merge"
	StatusLineModeReplace StatusLineMode = "replace"
)

type RuntimeConf struct {
	StatusLine StatusLineRuntimeConf `yaml:"-"`
}

type StatusLineRuntimeConf struct {
	Mode StatusLineMode `yaml:"-"`
}

func NormalizeStatusLineMode(raw string) StatusLineMode {
	return StatusLineMode(strings.ToLower(strings.TrimSpace(raw)))
}

func (m StatusLineMode) IsValid() bool {
	switch m {
	case StatusLineModeKeep, StatusLineModeMerge, StatusLineModeReplace:
		return true
	default:
		return false
	}
}
