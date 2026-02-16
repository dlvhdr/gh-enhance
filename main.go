package main

import (
	_ "embed"
	"os"

	"github.com/dlvhdr/gh-enhance/cmd/enhance"
)

func main() {
	if err := enhance.Execute(); err != nil {
		os.Exit(1)
	}
}
