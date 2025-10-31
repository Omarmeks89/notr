package notr

import (
	"testing"

	"github.com/Omarmeks89/notr/internal/tests"
)

func TestAll(t *testing.T) {
	tests.AnalyzeTestData(t, NewAnalyzer())
}
