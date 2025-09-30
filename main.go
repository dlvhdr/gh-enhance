package main

import (
	_ "embed"
	"os"

	goversion "github.com/caarlos0/go-version"

	"github.com/dlvhdr/gh-enhance/cmd/enhance"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
	builtBy = ""
)

func main() {
	if err := enhance.Execute(buildVersion(version, commit, date, builtBy)); err != nil {
		os.Exit(1)
	}
}

const website = "https://gh-dash.dev/enhance"

//go:embed internal/tui/art/logo.txt
var asciiArt string

func buildVersion(version, commit, date, builtBy string) goversion.Info {
	return goversion.GetVersionInfo(
		goversion.WithAppDetails("ENHANCE", "A Blazingly Fast Terminal UI for GitHub Actions", website),
		goversion.WithASCIIName(asciiArt),
		func(i *goversion.Info) {
			if commit != "" {
				i.GitCommit = commit
			}
			if date != "" {
				i.BuildDate = date
			}
			if version != "" {
				i.GitVersion = version
			}
			if builtBy != "" {
				i.BuiltBy = builtBy
			}
		},
	)
}
