package claude

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// mdSuffix is the markdown extension shared by every rule and prose surface.
const mdSuffix = ".md"

// relocatedRuleNames returns the file names of the rules the compiler moved out
// of rulecond.ClaudeRulesRelDir, derived from the compilation's own body set.
//
// Reading the compilation keeps the set from drifting: a rule that goes back to
// `always` or `paths-scoped` stops producing a relocated body and therefore
// stops being rewritten in the same edit that restores its baseline file.
func (s *claudeConditionalSurface) relocatedRuleNames() []string {
	if s == nil {
		return nil
	}
	return s.relocated
}

// rewriteRelocatedRuleReferences repoints every markdown reference to a
// relocated rule at the one place the Claude surface is finalized.
//
// A relocated rule has no file under rulecond.ClaudeRulesRelDir, so a skill,
// agent, command, or root doc that still names that path tells the model to
// read a file the installer never wrote. The rewrite runs over the assembled
// mapping set rather than inside each emitter because Claude markdown is
// produced by four independent paths — the embedded content copy, the workflow
// skill extractor, the router template, and the CLAUDE.md marker injector — and
// a per-emitter rewrite would have to be repeated in all four and would be
// missed by the next one added.
//
// Only markdown is rewritten. The compiled manifest and settings.json are
// byte-pinned machine surfaces that address rules by name, never by baseline
// path, so a prose path rewrite has nothing to do there.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: the single finalization seam for relocated rule references — every Claude markdown emitter converges on prepareFiles, which is why this rewrite is stated once.
// @AX:REASON: Sprinkling the rewrite into the emitters makes the reference contract depend on which emitter produced a file, and a relocated rule then keeps a dangling `.claude/rules/autopus/<name>.md` reference on whichever surface was forgotten.
func (s *claudeConditionalSurface) rewriteRelocatedRuleReferences(files []adapter.FileMapping) {
	names := s.relocatedRuleNames()
	if len(names) == 0 {
		return
	}

	pairs := make([]string, 0, 2*len(names))
	for _, name := range names {
		pairs = append(pairs,
			path.Join(rulecond.ClaudeRulesRelDir, name),
			path.Join(rulecond.BodyRootRelPath, name))
	}
	replacer := strings.NewReplacer(pairs...)

	for i := range files {
		if !strings.HasSuffix(filepath.ToSlash(files[i].TargetPath), mdSuffix) {
			continue
		}
		before := string(files[i].Content)
		after := replacer.Replace(before)
		if after == before {
			continue
		}
		files[i].Content = []byte(after)
		files[i].Checksum = checksum(after)
	}
}
