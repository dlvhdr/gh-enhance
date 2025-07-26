package main

import (
	"fmt"
	slog "log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log/v2"
	"github.com/spf13/cobra"

	"github.com/dlvhdr/gh-enhance/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:     "gh enhance",
	Short:   "In search of a better name",
	Version: "0.0.1",
	Args:    cobra.MaximumNArgs(1),
}

func init() {
	var loggerFile *os.File
	_, debug := os.LookupEnv("DEBUG")

	if debug {
		var fileErr error
		newConfigFile, fileErr := os.OpenFile("debug.log",
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if fileErr == nil {
			// log.SetColorProfile(term.TrueColor)
			log.SetOutput(newConfigFile)
			log.SetTimeFormat(time.Kitchen)
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

	rootCmd.Flags().BoolP(
		"help",
		"h",
		false,
		"help for gh-enhance",
	)

	rootCmd.Run = func(_ *cobra.Command, args []string) {
		if len(args) > 0 {
			p := tea.NewProgram(tui.NewModel(repo, args[0]), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				log.Error("failed starting program", "err", err)
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Usage: -R owner/repo 15623", "args", args)
			os.Exit(1)
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
