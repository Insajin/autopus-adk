package companionmanifest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestPublicKeyReceiptTrustAPI_ExposesOnlyBundleConstructor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	constructors := make([]*ast.FuncDecl, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			token.NewFileSet(),
			entry.Name(),
			nil,
			0,
		)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			if function.Name.Name == "VerifyPublicKeyReceipt" {
				t.Fatal("unsafe self-consistency key-returning API remains exported")
			}
			if ast.IsExported(function.Name.Name) && returnsTrustedReceipt(function.Type.Results) {
				constructors = append(constructors, function)
			}
		}
	}
	if len(constructors) != 1 ||
		constructors[0].Name.Name != "VerifyConfiguredPublicKeyReceiptBundle" {
		t.Fatalf("public trust constructors = %v", functionNames(constructors))
	}
	if hasByteSliceParameter(constructors[0].Type.Params) {
		t.Fatal("public trust constructor accepts independently selected byte slices")
	}
}

func returnsTrustedReceipt(results *ast.FieldList) bool {
	if results == nil {
		return false
	}
	for _, field := range results.List {
		if name, ok := field.Type.(*ast.Ident); ok && name.Name == "TrustedPublicKeyReceipt" {
			return true
		}
	}
	return false
}

func hasByteSliceParameter(params *ast.FieldList) bool {
	if params == nil {
		return false
	}
	for _, field := range params.List {
		array, ok := field.Type.(*ast.ArrayType)
		if !ok || array.Len != nil {
			continue
		}
		if element, ok := array.Elt.(*ast.Ident); ok && element.Name == "byte" {
			return true
		}
	}
	return false
}

func functionNames(functions []*ast.FuncDecl) []string {
	names := make([]string, 0, len(functions))
	for _, function := range functions {
		names = append(names, function.Name.Name)
	}
	return names
}
