// Package e2e provides user-facing scenario-based E2E test infrastructure.
package e2e

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// cobraCommand holds extracted Cobra command information.
type cobraCommand struct {
	Use    string
	Short  string
	HasRun bool // true if the command has a Run or RunE handler (leaf command)
}

// @AX:NOTE [AUTO] @AX:REASON: public API boundary — go/ast-based Cobra extractor; only leaf commands (HasRun=true) are emitted; fan_in=1 (pkg/setup/scenarios.go)
// ExtractCobra scans Go source files in the given directory for Cobra command
// definitions and returns a list of scenarios for leaf commands.
func ExtractCobra(dir string) ([]Scenario, error) {
	commands, err := scanCobraCommands(dir)
	if err != nil {
		return nil, err
	}

	var scenarios []Scenario
	for i, cmd := range commands {
		if !cmd.HasRun {
			continue // skip parent commands without a Run handler
		}
		s := Scenario{
			Number:       i + 1,
			ID:           cmd.Use,
			Description:  cmd.Short,
			Command:      cmd.Use,
			Precondition: "N/A",
			Env:          "N/A",
			Expect:       "exit 0",
			Verify:       []string{"exit_code(0)"},
			Depends:      "N/A",
			Status:       "active",
		}
		scenarios = append(scenarios, s)
	}
	return scenarios, nil
}

// scanCobraCommands walks Go source files and extracts cobra.Command literals.
func scanCobraCommands(dir string) ([]cobraCommand, error) {
	var cmds []cobraCommand

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() {
			// Skip vendor, testdata, and hidden directories.
			base := info.Name()
			if base == "vendor" || base == "testdata" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileCmds, err := extractFromFile(path)
		if err != nil {
			return nil // skip unparseable files
		}
		cmds = append(cmds, fileCmds...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cmds, nil
}

// extractFromFile parses a single Go file and extracts cobra.Command literals.
func extractFromFile(path string) ([]cobraCommand, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	var cmds []cobraCommand
	ast.Inspect(f, func(n ast.Node) bool {
		comp, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// Check if it's a cobra.Command composite literal.
		if !isCobraCommand(comp.Type) {
			return true
		}

		cmd := cobraCommand{}
		for _, elt := range comp.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Use":
				if lit, ok := kv.Value.(*ast.BasicLit); ok {
					cmd.Use = strings.Trim(lit.Value, `"`)
					// Use only the first word (command name, not args).
					if idx := strings.IndexByte(cmd.Use, ' '); idx > 0 {
						cmd.Use = cmd.Use[:idx]
					}
				}
			case "Short":
				if lit, ok := kv.Value.(*ast.BasicLit); ok {
					cmd.Short = strings.Trim(lit.Value, `"`)
				}
			case "Run", "RunE":
				cmd.HasRun = true
			}
		}

		if cmd.Use != "" {
			cmds = append(cmds, cmd)
		}
		return true
	})

	return cmds, nil
}

// isCobraCommand reports whether the expression refers to cobra.Command.
func isCobraCommand(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "cobra" && sel.Sel.Name == "Command"
}
