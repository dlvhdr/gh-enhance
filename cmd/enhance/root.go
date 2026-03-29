package enhance

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	slog "log"
	"net/url"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/cli/go-gh"
	"github.com/spf13/cobra"

	"github.com/charmbracelet/fang"
	"github.com/dlvhdr/gh-enhance/internal/tui"
	"github.com/dlvhdr/gh-enhance/internal/version"
)

//go:embed logo.txt
var asciiArt string
var logoWithTagline = lipgloss.NewStyle().Foreground(lipgloss.Green).Render(asciiArt)

var rootCmd = &cobra.Command{
	Use:   "gh enhance [<PR URL> | <run URL> | <number>] [flags]",
	Long:  logoWithTagline,
	Short: "A Blazingly Fast Terminal UI for GitHub Actions",
	Args:  cobra.MinimumNArgs(1),
	Example: `# look up via a full URL to a GitHub PR
 gh enhance https://github.com/dlvhdr/gh-dash/pull/767

 # look up via a PR number when inside a clone of dlvhdr/gh-dash
 # will look at checks of https://github.com/dlvhdr/gh-dash/pull/767
 gh enhance 767

 # look up via a full URL to a GitHub Actions run
 gh enhance https://github.com/dlvhdr/gh-dash/actions/runs/23687980056

 # look up via a run ID (--run disambiguates from PR numbers)
 gh enhance 23687980056 --run`,
}

func Execute() error {
	themeFunc := fang.WithColorSchemeFunc(func(
		ld lipgloss.LightDarkFunc,
	) fang.ColorScheme {
		def := fang.DefaultColorScheme(ld)
		def.DimmedArgument = ld(lipgloss.Black, lipgloss.White)
		def.Codeblock = ld(lipgloss.Color("#F1EFEF"), lipgloss.Color("#141417"))
		def.Title = lipgloss.Green
		def.Command = lipgloss.Green
		def.Program = lipgloss.Green
		return def
	})
	return fang.Execute(context.Background(), rootCmd, themeFunc, fang.WithVersion(version.Version))
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

	rootCmd.SetVersionTemplate(
		logoWithTagline + "\n\n" + `enhance {{printf "version %s\n" .Version}}`,
	)

	var repo string
	var number string

	rootCmd.PersistentFlags().StringVarP(
		&repo,
		"repo",
		"R",
		"",
		`[HOST/]OWNER/REPO   Select another repository using the [HOST/]OWNER/REPO format`,
	)

	rootCmd.Flags().Bool(
		"flat",
		false,
		"passing this flag will present checks as a flat list",
	)

	rootCmd.Flags().Bool(
		"run",
		false,
		"treat the numeric argument as a workflow run ID instead of a PR number",
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
		"help for enhance",
	)

	usage := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().
			Bold(true).
			Render("Usage:")+
			" `"+
			lipgloss.NewStyle().
				Foreground(lipgloss.Green).
				Render("gh enhance")+
			" https://github.com/owner/repo/pull/15623`.",
		"Run "+
			lipgloss.NewStyle().
				Background(lipgloss.Color("#141417")).
				Render("`gh enhance --help`")+
			" for help and examples.\n")

	rootCmd.RunE = func(_ *cobra.Command, args []string) error {
		if len(args) == 0 {
			fmt.Print(usage)
			return errors.New("no PR passed")
		}

		isRunMode := false
		var runID string

		url, err := url.Parse(args[0])
		if err == nil && url.Hostname() == "github.com" {
			parts := strings.Split(url.Path, "/")

			// Detect /owner/repo/actions/runs/{id} URLs
			if len(parts) >= 5 && parts[3] == "actions" && parts[4] == "runs" && len(parts) >= 6 {
				repo = parts[1] + "/" + parts[2]
				runID = parts[5]
				isRunMode = true
			} else if len(parts) >= 5 {
				// Existing PR URL handling: /owner/repo/pull/{number}
				repo = parts[1] + "/" + parts[2]
				number = parts[4]
			} else {
				fmt.Print(usage)
				return errors.New("bad URL passed")
			}
		}

		if repo == "" {
			r, err := gh.CurrentRepository()
			if err == nil {
				repo = r.Owner() + "/" + r.Name()
			}
		}

		// Check --run flag for bare numeric IDs
		runFlag, _ := rootCmd.Flags().GetBool("run")
		if runFlag {
			if isRunMode {
				// Already parsed a run URL, --run flag is redundant but not an error
			} else if number != "" {
				// A PR URL was parsed but --run was also passed
				return errors.New("cannot use --run flag with a PR URL")
			} else {
				if _, err := strconv.Atoi(args[0]); err != nil {
					fmt.Print(usage)
					return errors.New("run ID is not a number")
				}
				runID = args[0]
				isRunMode = true
			}
		}

		if !isRunMode && number == "" {
			if _, err := strconv.Atoi(args[0]); err != nil {
				fmt.Print(usage)
				return errors.New("PR number is not a number")
			} else {
				number = args[0]
			}
		}

		flat, err := rootCmd.Flags().GetBool("flat")
		if err != nil {
			return err
		}

		opts := tui.ModelOpts{Flat: flat}
		if isRunMode {
			opts.RunID = runID
		}

		p := tea.NewProgram(tui.NewModel(repo, number, opts))
		if _, err := p.Run(); err != nil {
			log.Error("failed starting program", "err", err)
			fmt.Println(err)
			os.Exit(1)
		}
		return nil
	}
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
