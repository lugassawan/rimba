package nolateexport_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/lugassawan/rimba/tools/lint/nolateexport"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, nolateexport.Analyzer, "example")
}
