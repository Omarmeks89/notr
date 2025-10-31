package main

import (
	"github.com/Omarmeks89/notr/pkg/notr"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(notr.NewAnalyzer())
}
