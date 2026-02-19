// Package nolateconst defines an analyzer that forbids package-level const
// declarations that appear after any func or method declaration. All consts
// should be grouped before functions.
package nolateconst

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports const declarations that appear after function declarations.
var Analyzer = &analysis.Analyzer{
	Name: "nolateconst",
	Doc:  "forbids package-level const declarations after function declarations",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		seenFunc := false
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				seenFunc = true
			case *ast.GenDecl:
				if d.Tok == token.CONST && seenFunc {
					pass.Reportf(d.Pos(), "const declaration should appear before all function declarations")
				}
			}
		}
	}
	return nil, nil
}
