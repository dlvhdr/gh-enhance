package main

import (
	"fmt"
	slog "log"
	"net/url"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/log/v2"
	"github.com/cli/go-gh"
	"github.com/spf13/cobra"

	"github.com/dlvhdr/gh-enhance/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:     "gh enhance [<url> | <number>] [flags]",
	Short:   "",
	Version: "0.0.1",
	Args:    cobra.ExactArgs(1),
}

func init() {
	var loggerFile *os.File
	_, debug := os.LookupEnv("DEBUG")

	if debug {
		var fileErr error
		newConfigFile, fileErr := os.OpenFile("debug.log",
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		if fileErr == nil {
			log.SetColorProfile(colorprofile.TrueColor)
			log.SetOutput(newConfigFile)
			log.SetTimeFormat("15:04:05.000")
			log.SetReportCaller(true)
			log.SetLevel(log.DebugLevel)
			log.Debug("Logging to debug.log")
		} else {
			loggerFile, _ = tea.LogToFile("debug.log", "debug")
			slog.Print("Failed setting up logging", fileErr)
		}
	} else {
		log.SetOutput(os.Stderr)
		log.SetLevel(log.FatalLevel)
	}

	if loggerFile != nil {
		defer loggerFile.Close()
	}

	var repo string
	var number string

	rootCmd.PersistentFlags().StringVarP(
		&repo,
		"repo",
		"R",
		"",
		`[HOST/]OWNER/REPO   Select another repository using the [HOST/]OWNER/REPO format`,
	)

	rootCmd.SetVersionTemplate(`gh-enhance {{printf "version %s\n" .Version}}`)

	rootCmd.Flags().Bool(
		"debug",
		false,
		"passing this flag will allow writing debug output to debug.log",
	)

	rootCmd.Flags().Uint64P(
		"attempt",
		"a",
		0,
		"The attempt number of the workflow run",
	)

	rootCmd.Flags().BoolP(
		"help",
		"h",
		false,
		"help for gh-enhance",
	)

	rootCmd.Run = func(_ *cobra.Command, args []string) {
		url, err := url.Parse(args[0])
		if err == nil && url.Hostname() == "github.com" {
			parts := strings.Split(url.Path, "/")
			if len(parts) < 5 {
				exitWithUsage()
			}

			repo = parts[1] + "/" + parts[2]
			number = parts[4]
		}

		if repo == "" {
			r, err := gh.CurrentRepository()
			if err == nil {
				repo = r.Owner() + "/" + r.Name()
			}
		}

		if number == "" {
			number = args[0]
		}

		p := tea.NewProgram(tui.NewModel(repo, number), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Error("failed starting program", "err", err)
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func exitWithUsage() {
	fmt.Println("Usage: -R owner/repo 15623 or URL to a PR")
	os.Exit(1)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
