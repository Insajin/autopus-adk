package design

import (
	"context"
	"net"
	"strings"
	"testing"
)

// --- fetch.go helpers ---

func TestResolveHostForDial_DirectIP(t *testing.T) {
	// Direct IPv4 — returns as-is, no resolver call.
	ips, err := resolveHostForDial(context.Background(), "93.184.216.34", nil)
	if err != nil {
		t.Fatalf("direct IP: %v", err)
	}
	if len(ips) != 1 || ips[0].String() != "93.184.216.34" {
		t.Errorf("direct IP result = %v", ips)
	}
}

func TestResolveHostForDial_IPv4MappedBlocked(t *testing.T) {
	// IPv4-mapped address in IPv6 notation (::ffff:x.x.x.x) must be blocked.
	_, err := resolveHostForDial(context.Background(), "::ffff:192.0.2.1", nil)
	if err == nil {
		t.Error("IPv4-mapped address must be rejected")
	}
}

func TestResolveHostForDial_ResolverUsed(t *testing.T) {
	fakeIP := net.ParseIP("93.184.216.34")
	resolver := func(_ context.Context, _ string) ([]net.IP, error) {
		return []net.IP{fakeIP}, nil
	}
	ips, err := resolveHostForDial(context.Background(), "example.com", resolver)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if len(ips) == 0 || !ips[0].Equal(fakeIP) {
		t.Errorf("resolver ips = %v", ips)
	}
}

func TestResolveHostForDial_ResolverEmpty(t *testing.T) {
	resolver := func(_ context.Context, _ string) ([]net.IP, error) {
		return nil, nil
	}
	_, err := resolveHostForDial(context.Background(), "example.com", resolver)
	if err == nil {
		t.Error("empty resolver result must error")
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []net.IP{
		net.ParseIP("127.0.0.1"),   // loopback
		net.ParseIP("::1"),         // loopback IPv6
		net.ParseIP("10.0.0.1"),    // private
		net.ParseIP("192.168.1.1"), // private
		net.ParseIP("172.16.0.1"),  // private
		net.ParseIP("169.254.1.1"), // link-local unicast
		net.ParseIP("0.0.0.0"),     // unspecified
		nil,                        // nil IP
	}
	for _, ip := range blocked {
		if !isBlockedIP(ip) {
			t.Errorf("isBlockedIP(%v) = false, want true", ip)
		}
	}
	// Public routable IP must not be blocked.
	public := net.ParseIP("93.184.216.34")
	if isBlockedIP(public) {
		t.Errorf("isBlockedIP(public) = true, want false")
	}
}

func TestResolveRedirect(t *testing.T) {
	// Absolute redirect.
	got, err := resolveRedirect("https://example.com/a", "https://example.com/b")
	if err != nil || got != "https://example.com/b" {
		t.Errorf("abs redirect = %q, %v", got, err)
	}
	// Relative redirect resolved against base.
	got, err = resolveRedirect("https://example.com/a/b", "c")
	if err != nil || got != "https://example.com/a/c" {
		t.Errorf("rel redirect = %q, %v", got, err)
	}
	// Empty location must error.
	_, err = resolveRedirect("https://example.com/", "")
	if err == nil {
		t.Error("empty location must error")
	}
	// Whitespace-only location must error.
	_, err = resolveRedirect("https://example.com/", "   ")
	if err == nil {
		t.Error("whitespace location must error")
	}
}

// --- pack.go helpers ---

func TestContextToPackContext_Found(t *testing.T) {
	ctx := Context{
		Found:        true,
		SourcePath:   "DESIGN.md",
		BaselinePath: "design/tokens.md",
		Summary:      "Design context summary",
		Diagnostics: []Diagnostic{
			{Path: "foo.tsx", Category: CategorySensitivePath},
			{Category: CategoryMissingPath},
		},
	}
	pc := contextToPackContext(ctx)
	if !pc.Found || pc.SourcePath != "DESIGN.md" || pc.BaselinePath != "design/tokens.md" {
		t.Errorf("found context = %+v", pc)
	}
	if pc.SkipReason != "" {
		t.Errorf("found context should not have skip reason, got %q", pc.SkipReason)
	}
	if len(pc.Diagnostics) != 2 {
		t.Fatalf("diag count = %d, want 2", len(pc.Diagnostics))
	}
	// Path-prefixed diagnostic.
	if !strings.Contains(pc.Diagnostics[0], "foo.tsx:sensitive_path") {
		t.Errorf("diag[0] = %q", pc.Diagnostics[0])
	}
	// No-path diagnostic.
	if !strings.Contains(pc.Diagnostics[1], "missing_path") {
		t.Errorf("diag[1] = %q", pc.Diagnostics[1])
	}
}

func TestContextToPackContext_NotFound(t *testing.T) {
	ctx := Context{Found: false, SkipReason: SkipMissing}
	pc := contextToPackContext(ctx)
	if pc.Found || pc.SkipReason != "missing" {
		t.Errorf("not-found context = %+v", pc)
	}
}

func TestWriteRefList(t *testing.T) {
	var sb strings.Builder
	// Empty refs: prints "none".
	writeRefList(&sb, "Token refs", nil)
	if !strings.Contains(sb.String(), "Token refs: none") {
		t.Errorf("empty list = %q", sb.String())
	}
	sb.Reset()
	// Non-empty refs: lists paths.
	refs := []SourceRef{{Path: "tokens.ts", Kind: "token_or_theme"}, {Path: "theme.ts", Kind: "token_or_theme"}}
	writeRefList(&sb, "Token refs", refs)
	if !strings.Contains(sb.String(), "tokens.ts") || !strings.Contains(sb.String(), "theme.ts") {
		t.Errorf("non-empty list = %q", sb.String())
	}
}

func TestAppendMissing(t *testing.T) {
	list := appendMissing(nil, "a")
	if len(list) != 1 || list[0] != "a" {
		t.Errorf("appendMissing nil = %v", list)
	}
	list = appendMissing(list, "a") // duplicate
	if len(list) != 1 {
		t.Errorf("duplicate not deduplicated = %v", list)
	}
	list = appendMissing(list, "b")
	if len(list) != 2 || list[1] != "b" {
		t.Errorf("appendMissing b = %v", list)
	}
}

func TestPackMarkdown_NoDesignContext(t *testing.T) {
	p := Pack{
		DesignContext: PackContext{Found: false, SkipReason: "missing"},
		SetupGaps:     []string{"figma_token_missing"},
	}
	md := p.Markdown()
	if !strings.Contains(md, "## Design Source Pack") {
		t.Errorf("markdown header missing: %q", md)
	}
	if !strings.Contains(md, "skipped (missing)") {
		t.Errorf("skip reason missing: %q", md)
	}
	if !strings.Contains(md, "figma_token_missing") {
		t.Errorf("setup gap missing: %q", md)
	}
	// Sections without data show "none".
	if !strings.Contains(md, "Figma refs: none") {
		t.Errorf("figma none missing: %q", md)
	}
}

func TestPackMarkdown_WithDesignContextAndBaselinePath(t *testing.T) {
	p := Pack{
		DesignContext: PackContext{Found: true, SourcePath: "DESIGN.md", BaselinePath: "design/tokens.md"},
		TokenRefs:     []SourceRef{{Path: "tokens.ts"}},
		FigmaRefs:     []FigmaRef{{SourcePath: "DESIGN.md", Kind: "design", URLHash: "abc123"}},
	}
	md := p.Markdown()
	if !strings.Contains(md, "Source of truth: design/tokens.md") {
		t.Errorf("baseline path missing: %q", md)
	}
	if !strings.Contains(md, "tokens.ts") {
		t.Errorf("token ref missing: %q", md)
	}
	if !strings.Contains(md, "abc123") {
		t.Errorf("figma hash missing: %q", md)
	}
}

func TestCollectDeclaredSourceRef_ParentTraversal(t *testing.T) {
	root := t.TempDir()
	pack := &Pack{}
	// Parent traversal path must be silently ignored.
	err := collectDeclaredSourceRef(root, "../evil", 10, pack)
	if err != nil {
		t.Errorf("parent traversal: %v", err)
	}
	if len(pack.TokenRefs)+len(pack.ComponentRefs)+len(pack.ScreenshotRefs) != 0 {
		t.Error("parent traversal must not add refs")
	}
}

func TestCollectDeclaredSourceRef_GlobSkipped(t *testing.T) {
	root := t.TempDir()
	pack := &Pack{}
	// Path containing glob chars must be silently ignored.
	err := collectDeclaredSourceRef(root, "src/**/*.tsx", 10, pack)
	if err != nil {
		t.Errorf("glob path: %v", err)
	}
}

func TestCollectDeclaredSourceRef_FileAdded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/tokens/colors.ts", "export const c = '#000'")
	pack := &Pack{}
	err := collectDeclaredSourceRef(root, "src/tokens/colors.ts", 10, pack)
	if err != nil {
		t.Fatalf("file ref: %v", err)
	}
	if len(pack.TokenRefs) != 1 || pack.TokenRefs[0].Reason != "source_of_truth" {
		t.Errorf("token refs = %+v", pack.TokenRefs)
	}
}
