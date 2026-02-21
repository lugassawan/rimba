// Package maxparams defines an analyzer that reports functions with more than 7 parameters.
package maxparams

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const maxParams = 7

// Analyzer reports functions whose parameter count exceeds maxParams.
var Analyzer = &analysis.Analyzer{
	Name:     "maxparams",
	Doc:      "forbids functions with more than 7 parameters",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		var params *ast.FieldList
		var name string

		switch fn := n.(type) {
		case *ast.FuncDecl:
			params = fn.Type.Params
			name = fn.Name.Name
		case *ast.FuncLit:
			params = fn.Type.Params
			name = "function literal"
		}

		if params == nil {
			return
		}

		count := countParams(params)
		if count > maxParams {
			pass.Reportf(n.Pos(), "%s has %d parameters, max is %d", name, count, maxParams)
		}
	})

	return nil, nil
}

func countParams(fields *ast.FieldList) int {
	total := 0
	for _, f := range fields.List {
		if len(f.Names) == 0 {
			total++ // anonymous parameter (e.g. in interface methods)
		} else {
			total += len(f.Names)
		}
	}
	return total
}
