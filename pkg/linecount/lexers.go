package linecount

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// languageByExtension pins every extension the size limit applies to onto a
// Chroma lexer name. Names are resolved by exact registry lookup rather than
// Chroma's filename matcher, because that matcher picks between overlapping
// globs by priority: ".php" resolves to the bare PHP lexer, which assumes the
// whole file is PHP code and therefore reads "//" in the surrounding HTML as a
// comment. PHTML is the same PHP grammar delegating to HTML outside "<?php".
//
// Extensions Chroma has no lexer for are mapped onto the closest grammar that
// shares the comment syntax: Less is a CSS superset that SCSS covers, and
// Chroma ships no TSX or CommonJS/ESM variants.
var languageByExtension = map[string]string{
	".c":     "C",
	".cc":    "C++",
	".cjs":   "JavaScript",
	".cpp":   "C++",
	".cs":    "C#",
	".css":   "CSS",
	".cxx":   "C++",
	".go":    "Go",
	".h":     "C",
	".hpp":   "C++",
	".java":  "Java",
	".js":    "JavaScript",
	".jsx":   "react",
	".kt":    "Kotlin",
	".kts":   "Kotlin",
	".less":  "SCSS",
	".mjs":   "JavaScript",
	".php":   "PHTML",
	".py":    "Python",
	".rb":    "Ruby",
	".rs":    "Rust",
	".sass":  "Sass",
	".scss":  "SCSS",
	".sh":    "Bash",
	".swift": "Swift",
	".ts":    "TypeScript",
	".tsx":   "TypeScript",
	".vue":   "vue",
}

// Language reports the grammar backing an extension, and whether the extension
// is one the size limit applies to at all. Callers enforcing the limit can key
// off this instead of keeping a second copy of the extension list.
func Language(extension string) (string, bool) {
	language, ok := languageByExtension[strings.ToLower(extension)]
	return language, ok
}

// lexerFor resolves the lexer for a filename, or nil when the extension is not
// one the limit applies to. Construction happens once per language and is
// deferred until a file of that language is actually counted, so nothing but
// Chroma's lazy registry is paid for by a caller that never lexes.
func lexerFor(filename string) (chroma.Lexer, error) {
	language, ok := languageByExtension[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		return nil, nil
	}
	load, ok := lexerLoaders[language]
	if !ok {
		return nil, fmt.Errorf("linecount: no loader for language %q", language)
	}
	return load()
}

// lexerLoaders holds one memoised constructor per language in the table above.
var lexerLoaders = func() map[string]func() (chroma.Lexer, error) {
	loaders := make(map[string]func() (chroma.Lexer, error), len(languageByExtension))
	for _, language := range languageByExtension {
		if _, done := loaders[language]; done {
			continue
		}
		loaders[language] = sync.OnceValues(func() (chroma.Lexer, error) {
			return buildLexer(language, correctionsByLanguage)
		})
	}
	return loaders
}()

// buildLexer fetches a registry lexer and applies this package's corrections.
//
// A correction is a failure when it cannot be applied: an inert patch would
// silently restore the undercount it exists to prevent, so the error travels
// out to the caller instead. The behaviour tests cover every correction, which
// is where a Chroma upgrade that renames a state gets caught.
func buildLexer(language string, corrections map[string]correction) (chroma.Lexer, error) {
	base := lexers.Get(language)
	if base == nil {
		return nil, fmt.Errorf("linecount: chroma has no lexer %q", language)
	}
	fix, corrected := corrections[language]
	if !corrected {
		return base, nil
	}
	regexLexer, ok := base.(*chroma.RegexLexer)
	if !ok {
		return nil, fmt.Errorf("linecount: lexer %q is not rule-based, cannot correct it", language)
	}
	rules, err := regexLexer.Rules()
	if err != nil {
		return nil, fmt.Errorf("linecount: rules for %q: %w", language, err)
	}
	if _, exists := rules[fix.state]; !exists {
		return nil, fmt.Errorf("linecount: lexer %q has no state %q to correct", language, fix.state)
	}
	patched := rules.Clone()
	patched[fix.state] = append(append([]chroma.Rule{}, fix.rules...), patched[fix.state]...)

	config := *regexLexer.Config()
	config.Name = language + " (linecount)"
	config.Aliases, config.Filenames, config.AliasFilenames, config.MimeTypes = nil, nil, nil, nil
	lexer, err := chroma.NewLexer(&config, func() chroma.Rules { return patched })
	if err != nil {
		return nil, fmt.Errorf("linecount: correct %q: %w", language, err)
	}
	// Corrected lexers are not registered, so they need the global registry
	// explicitly in case a grammar delegates to another language.
	return lexer.SetRegistry(lexers.GlobalLexerRegistry), nil
}
