package enhance

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	slog "log"
	"net/url"
	"os"
	"regexp"
	"strconv"

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

// Regex patterns for GitHub URL parsing
var (
	prURLPattern = regexp.MustCompile(
		`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/pull/(?P<number>\d+)`,
	)
	runURLPattern = regexp.MustCompile(
		`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/actions/runs/(?P<runID>\d+)`,
	)
)

var rootCmd = &cobra.Command{
	Use:   "gh enhance [<PR URL> | <run URL> | <number>] [flags]",
	Long:  logoWithTagline,
	Short: "A Blazingly Fast Terminal UI for GitHub Actions",
	Args:  cobra.MinimumNArgs(0),
	Example: `# look up via a full URL to a GitHub PR
 gh enhance https://github.com/dlvhdr/gh-dash/pull/767

 # look up via a PR number when inside a clone of dlvhdr/gh-dash
 # will look at checks of https://github.com/dlvhdr/gh-dash/pull/767
 gh enhance 767

 # look up via a full URL to a GitHub Actions run
 gh enhance https://github.com/dlvhdr/gh-dash/actions/runs/23687980056

 # look up via a run ID (--run disambiguates from PR numbers)
 gh enhance --run 23687980056`,
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

	rootCmd.Flags().String(
		"run",
		"",
		"look up a workflow run by its numeric ID",
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
		isRunMode := false
		var runID string

		// Check --run flag first (string flag takes the run ID directly)
		runFlagVal, _ := rootCmd.Flags().GetString("run")
		if runFlagVal != "" {
			if _, err := strconv.Atoi(runFlagVal); err != nil {
				fmt.Print(usage)
				return errors.New("run ID is not a number")
			}
			runID = runFlagVal
			isRunMode = true

			if len(args) > 0 {
				return errors.New("cannot pass both --run and a positional argument")
			}
		}

		if !isRunMode && len(args) == 0 {
			fmt.Print(usage)
			return errors.New("no PR passed")
		}

		// Parse positional argument (URL or bare number)
		if !isRunMode && len(args) > 0 {
			arg := args[0]
			u, err := url.Parse(arg)
			if err == nil && u.Hostname() == "github.com" {
				if m := runURLPattern.FindStringSubmatch(u.Path); m != nil {
					repo = m[runURLPattern.SubexpIndex("owner")] + "/" +
						m[runURLPattern.SubexpIndex("repo")]
					runID = m[runURLPattern.SubexpIndex("runID")]
					isRunMode = true
				} else if m := prURLPattern.FindStringSubmatch(u.Path); m != nil {
					repo = m[prURLPattern.SubexpIndex("owner")] + "/" +
						m[prURLPattern.SubexpIndex("repo")]
					number = m[prURLPattern.SubexpIndex("number")]
				} else {
					fmt.Print(usage)
					return errors.New("bad URL passed")
				}
			} else {
				// Bare number — must be a PR number
				if _, err := strconv.Atoi(arg); err != nil {
					fmt.Print(usage)
					return errors.New("PR number is not a number")
				}
				number = arg
			}
		}

		if repo == "" {
			r, err := gh.CurrentRepository()
			if err == nil {
				repo = r.Owner() + "/" + r.Name()
			}
		}

		if repo == "" {
			fmt.Print(usage)
			return errors.New("could not determine repository; use -R owner/repo")
		}

		if !isRunMode && number == "" {
			fmt.Print(usage)
			return errors.New("no PR or run ID provided")
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
