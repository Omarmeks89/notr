package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

type (
	errPrinter struct {
		errors int
	}
)

func (ep *errPrinter) Errorf(_ string, args ...any) {
	ep.errors++
}

func AnalyzeTestData(t *testing.T, analyzer *analysis.Analyzer) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	var p errPrinter
	analysistest.Run(&p, filepath.Join(wd, "testdata/src"), analyzer, "./...")

	require.Zero(t, p.errors)
}
