//go:build ignore

// gen-ts-constants — Parses backend/shared/config.go and emits
// frontend/src/lib/constants.ts so that shared constants stay in sync.
//
// Usage: go run ./backend/scripts/gen-ts-constants.go
// Called automatically by build.sh before the frontend build.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

const (
	inputPath  = "backend/shared/config.go"
	outputPath = "frontend/src/lib/constants.ts"
)

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, inputPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		os.Exit(1)
	}

	var consts []constant
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || len(valueSpec.Names) != 1 || len(valueSpec.Values) != 1 {
				continue
			}
			name := valueSpec.Names[0].Name
			if !isExported(name) {
				continue
			}
			val := extractLiteral(valueSpec.Values[0])
			if val == "" {
				continue
			}
			consts = append(consts, constant{Name: name, Value: val})
		}
	}

	var sb strings.Builder
	sb.WriteString("// AUTO-GENERATED — do not edit. Run go run ./backend/scripts/gen-ts-constants.go to regenerate.\n")
	sb.WriteString("// Source: " + inputPath + "\n\n")
	for _, c := range consts {
		sb.WriteString(fmt.Sprintf("export const %s = %s;\n", c.Name, c.Value))
	}

	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated %s (%d constants)\n", outputPath, len(consts))
}

type constant struct {
	Name  string
	Value string
}

func isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

// extractLiteral returns the Go source representation of a constant value
// as a TypeScript-compatible literal (number, string, or array).
func extractLiteral(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		// Go octal literals (0644) must become TypeScript octal (0o644)
		if e.Kind == token.INT && len(e.Value) > 1 && e.Value[0] == '0' && e.Value[1] != '.' && e.Value[1] != 'x' && e.Value[1] != 'X' {
			return "0o" + e.Value[1:]
		}
		return e.Value
	case *ast.BinaryExpr:
		left := extractLiteral(e.X)
		right := extractLiteral(e.Y)
		if left == "" || right == "" {
			return ""
		}
		return fmt.Sprintf("(%s %s %s)", left, e.Op, right)
	case *ast.Ident:
		return e.Name
	case *ast.CompositeLit:
		var elems []string
		for _, elt := range e.Elts {
			if lit, ok := elt.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				elems = append(elems, lit.Value)
			}
		}
		if len(elems) == 0 {
			return "[]"
		}
		return "[" + strings.Join(elems, ", ") + "]"
	}
	return ""
}
