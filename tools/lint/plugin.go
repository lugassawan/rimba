// Package lint registers rimba's custom lint analyzers as a golangci-lint v2 module plugin.
package lint

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/lugassawan/rimba/tools/lint/nolateexport"
	"github.com/lugassawan/rimba/tools/lint/nolocalstruct"
)

func init() {
	register.Plugin("rimbalint", newPlugin)
}

func newPlugin(_ any) (register.LinterPlugin, error) {
	return &plugin{}, nil
}

type plugin struct{}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		nolocalstruct.Analyzer,
		nolateexport.Analyzer,
	}, nil
}

func (p *plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}
