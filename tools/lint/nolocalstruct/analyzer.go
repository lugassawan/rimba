// Package nolocalstruct defines an analyzer that forbids named struct type
// declarations inside function bodies. Structs should be declared at package level.
package nolocalstruct

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports named struct types declared inside function bodies.
var Analyzer = &analysis.Analyzer{
	Name:     "nolocalstruct",
	Doc:      "forbids named struct type declarations inside function bodies",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Visit function declarations and function literals.
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		}
		if body == nil {
			return
		}
		checkBlock(pass, body)
	})

	return nil, nil
}

func checkBlock(pass *analysis.Pass, block *ast.BlockStmt) {
	ast.Inspect(block, func(n ast.Node) bool {
		ds, ok := n.(*ast.DeclStmt)
		if !ok {
			return true
		}
		gd, ok := ds.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			return true
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := ts.Type.(*ast.StructType); isStruct {
				pass.Reportf(ts.Pos(), "named struct type %q should be declared at package level, not inside a function", ts.Name.Name)
			}
		}
		return true
	})
}
