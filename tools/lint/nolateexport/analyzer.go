// Package nolateexport defines an analyzer that forbids exported standalone
// functions (no receiver) that appear after unexported standalone functions.
// All exported functions should be grouped before unexported ones.
package nolateexport

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports exported standalone functions that appear after unexported ones.
var Analyzer = &analysis.Analyzer{
	Name: "nolateexport",
	Doc:  "forbids exported standalone functions after unexported standalone functions",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		seenUnexported := false
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			if fn.Name.Name == "init" {
				continue
			}
			if !ast.IsExported(fn.Name.Name) {
				seenUnexported = true
			} else if seenUnexported {
				pass.Reportf(fn.Pos(), "exported function %s should appear before unexported functions", fn.Name.Name)
			}
		}
	}
	return nil, nil
}
