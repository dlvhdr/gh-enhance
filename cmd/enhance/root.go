package enhance

import (
	"fmt"
	slog "log"
	"net/url"
	"os"
	"strings"

	goversion "github.com/caarlos0/go-version"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/log/v2"
	"github.com/cli/go-gh"
	"github.com/spf13/cobra"

	"github.com/dlvhdr/gh-enhance/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "gh enhance [<url> | <number>] [flags]",
	Short: "",
	Args:  cobra.ExactArgs(1),
}

func Execute(version goversion.Info) error {
	rootCmd.Version = version.String()
	return rootCmd.Execute()
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
			setDebugLogLevel()
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
		"flat",
		false,
		"passing this flag will present checks as a flat list",
	)

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

	rootCmd.SetVersionTemplate(`gh-enhance {{printf "version %s\n" .Version}}`)

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

		flat, err := rootCmd.Flags().GetBool("flat")
		if err != nil {
			log.Fatal("Cannot parse the flat flag", err)
		}

		p := tea.NewProgram(tui.NewModel(repo, number, tui.ModelOpts{Flat: flat}), tea.WithAltScreen())
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

func setDebugLogLevel() {
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	}

	log.Debug("log level set", "level", log.GetLevel())
}
