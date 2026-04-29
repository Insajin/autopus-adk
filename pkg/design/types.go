package design

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	// @AX:NOTE [AUTO]: Context line and byte limits are safety defaults for untrusted design prompt material.
	DefaultMaxContextLines = 80
	MaxImportBodyBytes     = 1024 * 1024
	MaxLocalContextBytes   = 1024 * 1024
)

type Options struct {
	Enabled         bool
	Paths           []string
	MaxContextLines int
	UIFileGlobs     []string
}

type Context struct {
	Found        bool
	SourcePath   string
	BaselinePath string
	Summary      string
	Diagnostics  []Diagnostic
	SkipReason   SkipReason
}

type SkipReason string

const (
	SkipDisabled SkipReason = "disabled"
	SkipMissing  SkipReason = "missing"
)

type DiagnosticCategory string

const (
	CategoryParentTraversal      DiagnosticCategory = "parent_traversal"
	CategoryOutsideRoot          DiagnosticCategory = "outside_root"
	CategorySymlinkEscape        DiagnosticCategory = "symlink_escape"
	CategoryUnsupportedExtension DiagnosticCategory = "unsupported_extension"
	CategorySensitivePath        DiagnosticCategory = "sensitive_path"
	CategoryMissingPath          DiagnosticCategory = "missing_path"
	CategoryUnsafeScheme         DiagnosticCategory = "unsafe_scheme"
	CategoryLocalHostname        DiagnosticCategory = "local_hostname"
	CategoryPrivateAddress       DiagnosticCategory = "private_address"
	CategoryFetchRejected        DiagnosticCategory = "fetch_rejected"
	CategoryUnsafeContent        DiagnosticCategory = "unsafe_content"
	CategoryBodyTooLarge         DiagnosticCategory = "body_too_large"
)

type Diagnostic struct {
	Path     string             `json:"path,omitempty"`
	Category DiagnosticCategory `json:"category"`
	Message  string             `json:"message,omitempty"`
}

type Resolver func(context.Context, string) ([]net.IP, error)

type ImportOptions struct {
	TrustLabel string
	Now        time.Time
	Resolver   Resolver
	HTTPClient *http.Client
}

type ImportResult struct {
	ArtifactDir string
	Rejected    bool
	Reasons     []string
}
