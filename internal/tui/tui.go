package tui

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"
	checks "github.com/dlvhdr/x/gh-checks"
	help "github.com/dlvhdr/x/help"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/tui/art"
	"github.com/dlvhdr/gh-enhance/internal/tui/scrollbar"
	"github.com/dlvhdr/gh-enhance/internal/tui/util"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type pane int

const (
	PaneRuns pane = iota
	PaneJobs
	PaneSteps
	PaneChecks
	PaneLogs
)

type model struct {
	client                  api.API
	width                   int
	height                  int
	prNumber                string
	repo                    string
	runID                   string // set when viewing a run directly (no PR)
	pr                      api.PR
	prWithChecks            api.PRWithChecks
	workflowRuns            []data.WorkflowRun
	accumulatedWorkflowRuns []data.WorkflowRun
	runsList                list.Model
	jobsListRunId           string
	jobsList                list.Model
	stepsList               list.Model
	checksList              list.Model
	logsViewport            viewport.Model
	numHighlights           int
	scrollbar               util.Model
	focusedPane             pane
	zoomedPane              *pane
	err                     error
	runsDelegate            list.ItemDelegate
	jobsDelegate            list.ItemDelegate
	stepsDelegate           list.ItemDelegate
	checksDelegate          list.ItemDelegate
	styles                  styles
	logsSpinner             spinner.Model
	logsInput               textinput.Model
	inProgressSpinner       spinner.Model
	flat                    bool
	lastTick                time.Time
	version                 string
	rateLimit               api.RateLimit
	lastFetched             time.Time
	helpOpen                bool
	help                    help.Model
	unfocusedLargePaneWidth int
	focusedLargePaneWidth   int
	smallScreenWidth        int
}

type ModelOpts struct {
	Flat     bool
	Repo     string
	PRNumber string // non-empty when in PR context
	RunID    string // non-empty when in run mode (no PR context)

	// For testing
	API                     api.API
	UnfocusedLargePaneWidth int
	FocusedLargePaneWidth   int
	SmallScreenWidth        int
}

func NewModel(opts ModelOpts) model {
	tint.NewDefaultRegistry()
	tint.SetTintID(tint.TintTokyoNightStorm.ID)
	theme := os.Getenv("ENHANCE_THEME")
	if theme != "" {
		tint.SetTintID(theme)
	}

	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Sum != "" {
		version = info.Main.Version
	}

	s := makeStyles()

	runsList, runsDelegate := newRunsDefaultList(s)
	runsList.Title = makePill(ListSymbol+" Runs", s.focusedPaneTitleStyle,
		s.colors.focusedColor)
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(defaultFocusedLargePaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList(s)
	jobsList.Title = makePill(ListSymbol+" Jobs", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(defaultUnfocusedLargePaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList(s)
	stepsList.Title = makePill(ListSymbol+" Steps", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(defaultUnfocusedLargePaneWidth)

	checksList, checksDelegate := newChecksDefaultList(s)
	checksList.Title = makePill(ListSymbol+" checks", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	checksList.SetStatusBarItemName("step", "checks")
	checksList.SetWidth(defaultUnfocusedLargePaneWidth)

	vp := viewport.New()
	vp.LeftGutterFunc = func(info viewport.GutterContext) string {
		return lipgloss.NewStyle().Foreground(s.colors.faintColor).Render(
			fmt.Sprintf(" %*d %s ", 5, info.Index+1,
				lipgloss.NewStyle().Foreground(s.colors.fainterColor).Render("│")))
	}
	vp.KeyMap.Right = rightKey
	vp.KeyMap.Left = leftKey

	vp.HighlightStyle = lipgloss.NewStyle().Foreground(s.tint.Black).Background(s.tint.Blue)
	vp.SelectedHighlightStyle = lipgloss.NewStyle().
		Foreground(s.tint.Black).
		Background(s.tint.BrightGreen)

	sb := scrollbar.NewVertical()
	sb.Style = sb.Style.Inherit(s.scrollbarStyle)
	sb.ThumbStyle = sb.ThumbStyle.Inherit(s.scrollbarThumbStyle)
	sb.TrackStyle = sb.TrackStyle.Inherit(s.scrollbarTrackStyle)

	ls := spinner.New(spinner.WithSpinner(LogsFrames))
	ls.Style = s.faintFgStyle

	cachedSpinner = NewClockSpinner(s)

	li := textinput.New()
	li.SetWidth(20)
	li.SetStyles(textinput.Styles{
		Cursor: textinput.CursorStyle{
			Color: s.colors.faintColor,
			Shape: tea.CursorBar,
			Blink: false,
		},
		Focused: textinput.StyleState{
			Text:        lipgloss.NewStyle(),
			Placeholder: s.faintFgStyle,
			Prompt:      s.faintFgStyle,
		},
		Blurred: textinput.StyleState{
			Text:        lipgloss.NewStyle(),
			Placeholder: s.faintFgStyle,
			Prompt:      s.faintFgStyle,
		},
	})
	li.SetVirtualCursor(true)
	li.Prompt = " "
	li.Placeholder = "Search..."

	ips := spinner.New(spinner.WithSpinner(InProgressFrames))
	ips.Style = lipgloss.NewStyle().Foreground(s.colors.warnColor)

	h := help.New()
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(s.colors.lightColor)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(s.tint.BrightWhite)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Blue)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(lipgloss.Blue)

	focusedPane := PaneRuns
	flat := opts.Flat
	if opts.RunID == "" && opts.PRNumber == "" {
		flat = false
	}
	if flat {
		focusedPane = PaneChecks
	}

	client := opts.API
	if opts.API == (api.API{}) {
		client = api.New()
	}

	unfocusedLargePaneWidth := opts.UnfocusedLargePaneWidth
	if unfocusedLargePaneWidth == 0 {
		unfocusedLargePaneWidth = defaultUnfocusedLargePaneWidth
	}
	focusedLargePaneWidth := opts.FocusedLargePaneWidth
	if focusedLargePaneWidth == 0 {
		focusedLargePaneWidth = defaultFocusedLargePaneWidth
	}
	smallScreenWidth := opts.SmallScreenWidth
	if smallScreenWidth == 0 {
		smallScreenWidth = defaultSmallScreen
	}

	m := model{
		client:                  client,
		jobsList:                jobsList,
		runsList:                runsList,
		stepsList:               stepsList,
		checksList:              checksList,
		prNumber:                opts.PRNumber,
		repo:                    opts.Repo,
		runID:                   opts.RunID,
		runsDelegate:            runsDelegate,
		jobsDelegate:            jobsDelegate,
		stepsDelegate:           stepsDelegate,
		checksDelegate:          checksDelegate,
		logsViewport:            vp,
		scrollbar:               sb,
		styles:                  s,
		logsSpinner:             ls,
		logsInput:               li,
		help:                    h,
		version:                 version,
		inProgressSpinner:       ips,
		flat:                    flat,
		focusedPane:             focusedPane,
		lastFetched:             time.Now(),
		focusedLargePaneWidth:   focusedLargePaneWidth,
		unfocusedLargePaneWidth: unfocusedLargePaneWidth,
		smallScreenWidth:        smallScreenWidth,
	}
	m.help.SetKeys(keys.FullHelp())
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	switch m.mode() {
	case ModeRun:
		return m.makeInitRunModeCmd()
	case ModePR:
		return m.makeInitPRCmd()
	case ModeRepo:
		return m.makeInitRepoModeCmd()
	}

	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	if _, ok := msg.(spinner.TickMsg); !ok {
		log.Info("got msg", "type", fmt.Sprintf("%T", msg))
	}
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)

	// `startIntervalFetching` is sent after the `refreshInterval` duration has elapsed.
	// At this point, `m.fetchPRChecksWithInterval()` checks if all checks have concluded.
	// If they did - it's a noop, otherwise we check at the interval.
	// `m.fetchPRChecksWithInterval` needs an up to date model, so this *cannot* be called
	// at the `m.makeInitCmd`. The up to date model is received by this `Update` func.
	case startIntervalFetching:
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case startRunIntervalFetching:
		cmds = append(cmds, m.fetchRunWithInterval())

	case repoModeRunsFetchedMsg, repoModeIntervalFetchMsg:
		var repoMsg repoModeRunsFetchedMsg
		if tickMsg, ok := msg.(repoModeIntervalFetchMsg); ok {
			cmds = append(cmds, m.makeFetchRepoChecksWithInterval())
			repoMsg = tickMsg.msg.(repoModeRunsFetchedMsg)
		} else {
			repoMsg = msg.(repoModeRunsFetchedMsg)
		}

		if repoMsg.Err != nil {
			log.Debug("error when fetching repo runs", "err", repoMsg.Err)
			m.err = repoMsg.Err
			msgCmd := tea.Printf("%s\nrepo=%s, \nOriginal error: %v\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Error when fetching runs of repo."), m.repo, repoMsg.Err)
			return m, tea.Sequence(msgCmd, tea.Quit)
		}

		log.Debug("got repoRunsFetchedMsg", "len(msg.Runs)", len(repoMsg.Runs))

		enrichedMsg := m.enrichRepoModeFetchedRunsWithExistingJobs(repoMsg)
		m.workflowRuns = enrichedMsg.Runs
		m.lastFetched = time.Now()
		m.runsList.StopSpinner()

		// repo runs are returned by the REST api which doesn't include each run's jobs,
		// so we need to restore run's jobs we already fetched in a different call
		cmds = append(cmds, m.onWorkflowRunsFetched()...)

	case runModeFetchedMsg, runModeIntervalTickMsg:
		var rmMsg runModeFetchedMsg
		if tickMsg, ok := msg.(runModeIntervalTickMsg); ok {
			cmds = append(cmds, m.makeFetchRunIntervalTickCmd())
			rmMsg = tickMsg.msg.(runModeFetchedMsg)
		} else {
			rmMsg = msg.(runModeFetchedMsg)
		}

		if rmMsg.err != nil {
			log.Debug("error when fetching run", "err", rmMsg.err)
			m.err = rmMsg.err
			msgCmd := tea.Printf("%s\nrepo=%s, runID=%s\nOriginal error: %v\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Workflow run not found."), m.repo, m.runID, rmMsg.err)
			return m, tea.Sequence(msgCmd, tea.Quit)
		}

		m.workflowRuns = rmMsg.runs
		m.lastFetched = time.Now()
		m.runsList.StopSpinner()
		cmds = append(cmds, m.onWorkflowRunsFetched()...)

	case prFetchedMsg:
		m.pr = msg.pr

	case prChecksFetchedMsg, prChecksIntervalTickMsg:
		var wrMsg prChecksFetchedMsg
		if tickMsg, ok := msg.(prChecksIntervalTickMsg); ok {
			wrMsg = tickMsg.msg.(prChecksFetchedMsg)
		} else {
			wrMsg = msg.(prChecksFetchedMsg)
		}
		m.rateLimit = wrMsg.rateLimit
		if wrMsg.err != nil && wrMsg.rateLimit.Remaining == 0 {
			log.Warn("rate limit reached, waiting", "rateLimit", wrMsg.rateLimit)
			return m, nil
		}

		if len(wrMsg.pr.Commits.Nodes) > 0 {
			log.Debug("workflow runs fetched", "fetched",
				len(wrMsg.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes))
		}

		m.prWithChecks = wrMsg.pr
		if _, ok := msg.(prChecksIntervalTickMsg); ok {
			cmds = append(cmds, m.fetchPRChecksWithInterval())
		}

		if len(wrMsg.pr.Commits.Nodes) > 0 {
			pageInfo := wrMsg.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.PageInfo
			if !pageInfo.HasPreviousPage {
				m.accumulatedWorkflowRuns = make([]data.WorkflowRun, 0)
			}

			if pageInfo.HasNextPage {
				m.accumulatedWorkflowRuns = m.mergeWorkflowRuns(wrMsg, m.accumulatedWorkflowRuns)
				log.Info("fetching next checks page", "pageInfo", pageInfo)
				cmds = append(cmds, m.makeGetNextPagePRChecksCmd(pageInfo.EndCursor))
			} else {
				m.workflowRuns = m.mergeWorkflowRuns(wrMsg, m.accumulatedWorkflowRuns)
				m.lastFetched = time.Now()
				m.checksList.StopSpinner()
				m.runsList.StopSpinner()
				log.Info("fetched all checks", "pageInfo", pageInfo)
				cmds = append(cmds, m.onWorkflowRunsFetched()...)
			}
		} else {
			m.stopSpinners()
		}

		if wrMsg.err != nil {
			log.Debug("error when fetching workflow runs", "err", wrMsg.err)
			m.err = wrMsg.err
			msgCmd := tea.Printf("%s\nrepo=%s, number=%s\nOriginal error: %v\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Pull request not found."), m.repo, m.prNumber, wrMsg.err)
			return m, tea.Sequence(msgCmd, tea.Quit)
		}

	case runJobsFetchedMsg:
		cmds = append(cmds, m.enrichRunWithJobs(msg)...)

	case workflowRunStepsFetchedMsg:
		cmds = append(cmds, m.enrichRunWithJobsStepsV2(msg)...)
		cmds = append(cmds, m.updateListsSpinners()...)

	case checkStepsFetchedMsg:
		cmds = append(cmds, m.enrichCheckWithSteps(msg)...)
		cmds = append(cmds, m.updateListsSpinners()...)

	case jobLogsFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			ji.logs = msg.logs
			ji.logsErr = msg.err
			ji.logsStderr = msg.stderr
			ji.loadingLogs = false
			currJob := m.getSelectedJobItem()
			if currJob != nil && currJob.job.Id == msg.jobId {
				cmds = append(cmds, m.renderJobLogs())
				m.goToErrorInLogs()
			}

			cmds = append(cmds, m.updateListsSpinners()...)
		}

	case checkRunOutputFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			if ji.job.Id == msg.jobId {
				ji.renderedText = msg.renderedText
				ji.loadingLogs = false
				currJob := m.getSelectedJobItem()
				if currJob != nil && currJob.job.Id == msg.jobId {
					cmds = append(cmds, m.renderJobLogs())
				}

				cmds = append(cmds, m.updateListsSpinners()...)
				break
			}
		}

	case reRunJobMsg:
		if msg.err != nil {
			log.Error("error rerunning job", "jobId", msg.jobId, "err", msg.err)
		}
		ji := m.getJobItemById(msg.jobId)
		if ji == nil {
			break
		}

		m.lastFetched = time.Now()
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case reRunRunMsg:
		if msg.err != nil {
			log.Error("error rerunning run", "jobId", msg.runId, "err", msg.err)
		}
		ri := m.getRunItemById(msg.runId)
		if ri == nil {
			break
		}

		m.lastFetched = time.Now()
		switch m.mode() {
		case ModePR:
			cmds = append(cmds, m.fetchPRChecksWithInterval())
		case ModeRepo:
			// not supported yet
		case ModeRun:
			cmds = append(cmds, m.startFetchingRunWithInterval())
		}

	case tea.WindowSizeMsg:
		log.Info("window size changed", "width", msg.Width, "height", msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		m.setHeights()
		m.setWidths()

		m.setFocusedPaneStyles()
	case tea.KeyPressMsg:
		if key.Matches(msg, quitKey) {
			log.Info("quitting", "msg", msg)
			return m, tea.Quit
		}

		log.Info("key pressed", "key", msg.String())
		if m.checksList.FilterState() == list.Filtering ||
			m.runsList.FilterState() == list.Filtering ||
			m.jobsList.FilterState() == list.Filtering ||
			m.stepsList.FilterState() == list.Filtering {
			// handled at the list Update func
			break
		}

		if m.logsInput.Focused() {
			if key.Matches(msg, applySearchKey) {
				ji := m.getSelectedJobItem()
				if ji != nil {
					m.logsViewport.SetContentLines(ji.unstyledLogs)
					highlights := regexp.MustCompile(
						m.logsInput.Value()).FindAllStringIndex(
						strings.Join(ji.unstyledLogs, "\n"), -1)
					m.numHighlights = len(highlights)
					m.logsViewport.SetHighlights(highlights)
					m.logsViewport.HighlightNext()
					m.logsInput.Blur()
				}
			} else {
				m.logsInput, cmd = m.logsInput.Update(msg)
				cmds = append(cmds, cmd)
				break
			}
		}

		if key.Matches(msg, modeKey) {
			// TODO: block if we're in repo mode
			m.flat = !m.flat
			if m.flat {
				m.focusedPane = PaneChecks
			} else {
				m.focusedPane = PaneRuns
			}
			cmds = append(cmds, m.onWorkflowRunsFetched()...)
		}

		if key.Matches(msg, zoomPaneKey) {
			if m.zoomedPane == nil {
				m.zoomedPane = &m.focusedPane
				m.setWidths()
			} else {
				m.zoomedPane = nil
			}
		}

		if key.Matches(msg, refreshAllKey) {
			newModel := NewModel(ModelOpts{Repo: m.repo, PRNumber: m.prNumber, RunID: m.runID})
			newModel.flat = m.flat
			newModel.focusedPane = m.focusedPane
			newModel.width = m.width
			newModel.height = m.height
			newModel.setHeights()
			newModel.setWidths()

			newModel.setFocusedPaneStyles()

			m.lastFetched = time.Now()
			log.Info(
				"refresh all",
				"repo",
				newModel.repo,
				"prNumber",
				newModel.prNumber,
				"runId",
				newModel.runID,
			)
			return newModel, newModel.Init()
		}

		if key.Matches(msg, rerunKey) {
			if m.focusedPane != PaneRuns && m.focusedPane != PaneJobs &&
				m.focusedPane != PaneChecks {
				break
			}

			ri := m.getSelectedRunItem()
			if ri == nil {
				break
			}

			if m.focusedPane == PaneRuns {
				cmds = append(cmds, m.rerunRun(ri.run.Id)...)
			} else {
				ji := m.getSelectedJobItem()
				if ji == nil {
					break
				}
				rid := ri.run.Id
				cmds = append(cmds, m.rerunJob(rid, ji.job.Id)...)
			}
		}

		if key.Matches(msg, helpKey) {
			m.helpOpen = !m.helpOpen
			m.setHeights()
		}

		if m.focusedPane == PaneLogs && key.Matches(msg, searchKey) {
			cmds = append(cmds, m.logsInput.Focus())
		}

		if key.Matches(msg, openPRKey) && m.prWithChecks.Url != "" {
			cmds = append(cmds, makeOpenUrlCmd(m.prWithChecks.Url))
		}

		if key.Matches(msg, nextPaneKey) {
			m.focusedPane = m.nextPane()
			m.zoomedPane = nil
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, prevPaneKey) {
			m.focusedPane = m.previousPane()
			m.zoomedPane = nil
			m.setFocusedPaneStyles()
		}

	case spinner.TickMsg:
		cachedSpinner, cmd = cachedSpinner.Update(msg)
		cmds = append(cmds, cmd)

		ji := m.getSelectedJobItem()
		if ji == nil || ji.loadingLogs {
			m.logsSpinner, cmd = m.logsSpinner.Update(msg)
			cmds = append(cmds, cmd)
		} else if ji.isStatusInProgress() {
			m.inProgressSpinner, cmd = m.inProgressSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		ci := m.getSelectedCheckItem()
		if ci == nil || ci.loadingLogs {
			m.logsSpinner, cmd = m.logsSpinner.Update(msg)
			cmds = append(cmds, cmd)
		} else if ci.isStatusInProgress() {
			m.inProgressSpinner, cmd = m.inProgressSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	switch m.focusedPane {
	case PaneChecks:
		before := m.getSelectedCheckItem()
		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.getSelectedCheckItem()
		if !areCheckItemsEqual(before, after) {
			cmds = append(cmds, m.onCheckChanged()...)
			cmds = append(cmds, m.updateListsSpinners()...)
		}
	case PaneRuns:
		before := m.getSelectedRunItem()
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.getSelectedRunItem()
		if !areRunItemsEqual(before, after) {
			cmds = append(cmds, m.onRunChanged()...)
		}
	case PaneJobs:
		before := m.getSelectedJobItem()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.getSelectedJobItem()
		if !areJobItemsEqual(before, after) {
			cmds = append(cmds, m.onJobChanged()...)
		}
	case PaneSteps:
		before := m.getSelectedStepItem()
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.getSelectedStepItem()
		if !areStepItemsEqual(before, after) {
			m.onStepChanged()
		}

	case PaneLogs:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(msg, gotoBottomKey) {
				m.logsViewport.GotoBottom()
			}

			if key.Matches(msg, gotoTopKey) {
				m.logsViewport.GotoTop()
			}

			if key.Matches(msg, nextSearchMatchKey) {
				m.logsViewport.HighlightNext()
			}

			if key.Matches(msg, prevSearchMatchKey) {
				m.logsViewport.HighlightPrevious()
			}

			if key.Matches(msg, cancelSearchKey) {
				m.logsInput.Blur()
				m.logsInput.Reset()
				m.numHighlights = 0
				m.logsViewport.ClearHighlights()
				ji := m.getSelectedJobItem()
				if ji != nil {
					m.logsViewport.SetContentLines(ji.renderedLogs)
				}
			}
		}
		m.logsViewport, cmd = m.logsViewport.Update(msg)

		cmds = append(cmds, cmd)
	}

	if _, ok := msg.(tea.KeyPressMsg); !ok && m.logsInput.Focused() {
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.setFocusedPaneStyles()

	m.scrollbar, cmd = m.scrollbar.Update(m.logsViewport)
	cmds = append(cmds, cmd)

	m.updateListsFiltersStates()

	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	var v tea.View
	if m.err != nil {
		log.Error("fatal error", "err", m.err)
		v.SetContent(m.err.Error())
		v.AltScreen = false
		return v
	}

	header := m.viewHeader()
	footer := m.viewFooter()

	panes := ""
	if m.flat {
		panes = m.viewFlatChecks()
	} else {
		panes = m.viewHierarchicalChecks()
	}

	rootStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height)
	appView := rootStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		panes,
		footer,
	))

	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(appView),
	}

	if m.helpOpen {
		helpView := m.help.View()
		row := m.height/4 - 2 // just a bit above the center
		col := m.width / 2
		col -= lipgloss.Width(helpView) / 2
		layers = append(
			layers,
			lipgloss.NewLayer(helpView).X(col).Y(row),
		)
	}

	comp := lipgloss.NewCompositor(layers...)
	v.AltScreen = true
	v.Content = comp.Render()
	return v
}

func (m *model) viewHierarchicalChecks() string {
	runsPane := makePointingBorder(m.paneStyle(PaneRuns).Render(m.runsList.View()))
	jobsPane := makePointingBorder(m.paneStyle(PaneJobs).Render(m.jobsList.View()))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.paneStyle(PaneSteps).Render(m.stepsList.View()))
	}

	panes := make([]string, 0)
	if m.zoomedPane != nil {
		switch *m.zoomedPane {
		case PaneRuns:
			panes = append(panes, runsPane)
		case PaneJobs:
			panes = append(panes, jobsPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			panes = append(panes, m.viewLogs())
		}
	} else if m.width != 0 && m.width <= m.smallScreenWidth {
		switch m.focusedPane {
		case PaneRuns:
			panes = append(panes, runsPane)
		case PaneJobs:
			panes = append(panes, jobsPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			break
		}
		panes = append(panes, m.viewLogs())
	} else {
		panes = append(panes, runsPane)
		panes = append(panes, jobsPane)
		panes = append(panes, stepsPane)
		panes = append(panes, m.viewLogs())
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panes...,
	)
}

func (m *model) viewFlatChecks() string {
	checksPane := makePointingBorder(m.paneStyle(PaneChecks).Render(m.checksList.View()))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.paneStyle(PaneSteps).Render(m.stepsList.View()))
	}

	panes := make([]string, 0)
	if m.zoomedPane != nil {
		switch *m.zoomedPane {
		case PaneChecks:
			panes = append(panes, checksPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			panes = append(panes, m.viewLogs())
		}
	} else if m.width != 0 && m.width <= m.smallScreenWidth {
		switch m.focusedPane {
		case PaneChecks:
			panes = append(panes, checksPane)
		case PaneSteps:
			panes = append(panes, stepsPane)
		case PaneLogs:
			break
		}
		panes = append(panes, m.viewLogs())
	} else {
		panes = append(panes, checksPane)
		panes = append(panes, stepsPane)
		panes = append(panes, m.viewLogs())
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		panes...,
	)
}

func (m *model) viewHeader() string {
	bgStyle := lipgloss.NewStyle().Background(m.styles.headerStyle.GetBackground())
	version := bgStyle.Height(lipgloss.Height(Logo)).Render(fmt.Sprintf(" \n %s", m.version))

	logoWidth := lipgloss.Width(Logo) + lipgloss.Width(version)
	logo := lipgloss.PlaceHorizontal(
		logoWidth,
		lipgloss.Right,
		bgStyle.Width(logoWidth).Render(
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				m.styles.logoStyle.Render(Logo),
				m.styles.faintFgStyle.Render(version),
			)))

	mode := m.mode()
	if mode == ModeRun {
		return m.viewRunModeHeader(bgStyle, logo, logoWidth)
	}

	if mode == ModeRepo {
		return m.viewRepoModeHeader(bgStyle, logo, logoWidth)
	}

	status := bgStyle.Render(m.viewCommitStatus(bgStyle))
	titleWidth := m.width - lipgloss.Width(status) - logoWidth -
		m.styles.headerStyle.GetHorizontalFrameSize()
	title := ""
	if m.pr.Title != "" {
		title = bgStyle.Width(titleWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
			m.viewRepo(titleWidth, bgStyle),
			m.viewPRName(titleWidth, bgStyle),
		))
	} else if m.mode() == ModePR {
		title = bgStyle.Width(titleWidth).
			Render(fmt.Sprintf("Loading %s PR #%s...", m.repo, m.prNumber))
	}

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, status, bgStyle.Render(title), logo))
}

func (m *model) viewRepo(width int, bgStyle lipgloss.Style) string {
	status := ""
	if m.pr.Merged {
		status = makePill(
			fmt.Sprintf("%s %s", MergedIcon, "Merged"),
			lipgloss.NewStyle().
				Foreground(m.styles.colors.darkerColor),
			m.styles.colors.mergedColor,
		)
	} else if m.pr.IsDraft {
		status = makePill(fmt.Sprintf("%s %s", DraftIcon, "Draft"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.whiteColor)
	} else if m.pr.Closed {
		status = makePill(fmt.Sprintf("%s %s", ClosedIcon, "Closed"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.errorColor)
	} else {
		status = makePill(
			fmt.Sprintf("%s %s", OpenIcon, "Open"),
			lipgloss.NewStyle().
				Foreground(m.styles.colors.darkerColor),
			m.styles.colors.successColor,
		)
	}

	return bgStyle.Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top,
		bgStyle.Render(status),
		bgStyle.Render(" "),
		bgStyle.Foreground(m.styles.colors.darkColor).Bold(true).Render(
			m.pr.Repository.NameWithOwner),
		bgStyle.Render(" "),
		bgStyle.Foreground(m.styles.colors.faintColor).Render(
			fmt.Sprintf("#%d", m.pr.Number)),
	))
}

func (m *model) viewPRName(width int, bgStyle lipgloss.Style) string {
	return bgStyle.Width(width).Bold(true).Render(m.pr.Title)
}

func (m *model) viewRunModeHeader(bgStyle lipgloss.Style, logo string, logoWidth int) string {
	contentWidth := m.width - logoWidth - m.styles.headerStyle.GetHorizontalFrameSize()

	if len(m.workflowRuns) == 0 {
		title := bgStyle.Width(contentWidth).
			Render(fmt.Sprintf("Loading %s run #%s...", m.repo, m.runID))
		return m.styles.headerStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left, bgStyle.Render(title), logo))
	}

	run := m.workflowRuns[0]

	// Show the run status as a pill
	statusText := ""
	var statusColor color.Color
	bucket := run.Bucket
	switch bucket {
	case data.CheckBucketPass:
		statusText = SuccessIcon + " Success"
		statusColor = m.styles.colors.successColor
	case data.CheckBucketFail:
		statusText = FailureIcon + " Failed"
		statusColor = m.styles.colors.errorColor
	case data.CheckBucketCancel:
		statusText = CanceledIcon + " Cancelled"
		statusColor = m.styles.colors.faintColor
	case data.CheckBucketPending:
		statusText = PendingIcon + " In Progress"
		statusColor = m.styles.colors.warnColor
	default:
		statusText = WaitingIcon + " Pending"
		statusColor = m.styles.colors.warnColor
	}
	status := makePill(statusText,
		lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), statusColor)

	repoName := bgStyle.Foreground(m.styles.colors.darkColor).Bold(true).Render(m.repo)
	separator := bgStyle.Foreground(m.styles.colors.faintColor).Render(" ⋅ ")
	runIdText := bgStyle.Foreground(m.styles.colors.faintColor).Render(
		fmt.Sprintf("run #%s", m.runID))

	topLine := bgStyle.Width(contentWidth).Render(lipgloss.JoinHorizontal(lipgloss.Top,
		bgStyle.Render(status),
		bgStyle.Render(" "),
		repoName,
		separator,
		runIdText,
	))

	// Show PR title and number when the run is against a PR, otherwise show
	// the run's display title to avoid duplicating the workflow name that
	// already appears in the run list.
	titleText := run.DisplayTitle
	if titleText == "" {
		titleText = run.Name
	}

	var bottomLine string
	if run.PRNumber > 0 {
		prLabel := bgStyle.Foreground(m.styles.colors.faintColor).Render(
			fmt.Sprintf("#%d", run.PRNumber))
		bottomLine = bgStyle.Width(contentWidth).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				bgStyle.Bold(true).Render(titleText),
				bgStyle.Render(" "),
				prLabel,
			))
	} else {
		bottomLine = bgStyle.Width(contentWidth).Bold(true).Render(titleText)
	}

	title := bgStyle.Width(contentWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
		topLine,
		bottomLine,
	))

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, bgStyle.Render(title), logo))
}

func (m *model) viewRepoModeHeader(bgStyle lipgloss.Style, logo string, logoWidth int) string {
	contentWidth := m.width - logoWidth - m.styles.headerStyle.GetHorizontalFrameSize()

	if len(m.workflowRuns) == 0 {
		title := bgStyle.Width(contentWidth).
			Render(fmt.Sprintf("Loading %s latest runs…", m.repo))
		return m.styles.headerStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left, title, logo))
	}

	title := bgStyle.Width(contentWidth).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		bgStyle.Width(contentWidth).Bold(true).Render(fmt.Sprintf("All %s workflows", m.repo)),
		bgStyle.Width(contentWidth).
			Foreground(m.styles.colors.faintColor).
			Render("Showing runs from all workflows"),
	))

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, title, logo))
}

func (m *model) viewFooter() string {
	bg := lipgloss.NewStyle().Background(m.styles.footerStyle.GetBackground())
	sFooter := m.styles.footerStyle.Width(m.width)

	mode := m.mode()
	if mode == ModeRun {
		return m.viewRunModeFooter(bg, sFooter)
	}

	if mode == ModeRepo {
		return m.viewRepoModeFooter(bg, sFooter)
	}

	if m.width == 0 || len(m.prWithChecks.Commits.Nodes) == 0 {
		return sFooter.Inherit(bg).Render("")
	}

	texts := make([]string, 0)

	contexts := m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts
	total := contexts.CheckRunCount + contexts.StatusContextCount
	totalText := ""
	if total > 0 {
		totalText = bg.Foreground(m.styles.colors.lightColor).Render(
			fmt.Sprintf("%d checks: ", total))
	}

	stats := checks.AccumulatedStats(
		contexts.CheckRunCountsByState,
		contexts.StatusContextCountsByState,
	)

	texts = m.appendStatTexts(
		texts, bg, stats.Failed, stats.InProgress, stats.Succeeded, stats.Skipped,
	)
	checksText := bg.Render(strings.Join(texts, bg.Render(", ")))
	return m.renderFooterLayout(bg, sFooter, m.isInProgress(), totalText, checksText)
}

func (m *model) viewRunModeFooter(bg lipgloss.Style, sFooter lipgloss.Style) string {
	if m.width == 0 || len(m.workflowRuns) == 0 {
		return sFooter.Inherit(bg).Render("")
	}

	run := m.workflowRuns[0]
	texts := make([]string, 0)

	totalJobs := len(run.Jobs)
	totalText := ""
	if totalJobs > 0 {
		totalText = bg.Foreground(m.styles.colors.lightColor).Render(
			fmt.Sprintf("%d jobs: ", totalJobs))
	}

	var failed, inProgress, succeeded, skipped int
	for _, job := range run.Jobs {
		switch job.Bucket {
		case data.CheckBucketFail:
			failed++
		case data.CheckBucketPending:
			inProgress++
		case data.CheckBucketPass:
			succeeded++
		case data.CheckBucketSkipping:
			skipped++
		}
	}

	texts = m.appendStatTexts(texts, bg, failed, inProgress, succeeded, skipped)
	checksText := bg.Render(strings.Join(texts, bg.Render(", ")))

	return m.renderFooterLayout(bg, sFooter, m.isRunModeInProgress(), totalText, checksText)
}

func (m *model) viewRepoModeFooter(bg lipgloss.Style, sFooter lipgloss.Style) string {
	return m.renderFooterLayout(bg, sFooter, true, fmt.Sprintf("Watching %s…", m.repo))
}

func (m *model) appendStatTexts(
	texts []string, bg lipgloss.Style,
	failed, inProgress, succeeded, skipped int,
) []string {
	if failed > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.errorColor).Render(
			fmt.Sprintf("%d failing", failed)))
	}
	if inProgress > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.warnColor).Render(
			fmt.Sprintf("%d in progress", inProgress)))
	}
	if succeeded > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.successColor).Render(
			fmt.Sprintf("%d successful", succeeded)))
	}
	if skipped > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.faintColor).Render(
			fmt.Sprintf("%d skipped", skipped)))
	}
	return texts
}

func (m *model) renderFooterLayout(
	bg, sFooter lipgloss.Style,
	isInProgress bool,
	additionalParts ...string,
) string {
	reFetchingIn := ""
	if isInProgress {
		until := time.Until(m.lastFetched.Add(refreshInterval)).Truncate(time.Second).Seconds()
		untilStr := fmt.Sprintf("in %ds", int(until))
		if until <= 0 {
			untilStr = "now..."
		}
		reFetchingIn = bg.Padding(0, 1).
			Foreground(m.styles.colors.faintColor).
			Render(fmt.Sprintf("refreshing %s", untilStr))
	}
	help := m.styles.helpButtonStyle.Render("? help")

	partsWidth := 0
	for _, part := range additionalParts {
		partsWidth += lipgloss.Width(part)
	}

	gap := bg.Render(
		strings.Repeat(
			" ",
			max(0, m.width-partsWidth-lipgloss.Width(reFetchingIn)-lipgloss.Width(help)-
				m.styles.footerStyle.GetHorizontalFrameSize()),
		),
	)

	footer := lipgloss.JoinHorizontal(lipgloss.Top, additionalParts...)
	return sFooter.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, footer, gap, reFetchingIn, help))
}

func (m *model) isRunModeInProgress() bool {
	if len(m.workflowRuns) == 0 {
		return true
	}

	if m.flat {
		if ci := m.getSelectedCheckItem(); ci.isStatusInProgress() {
			return true
		}
	} else if ri := m.getSelectedRunItem(); ri != nil && ri.HasNotConcluded() {
		return true
	}

	for _, job := range m.workflowRuns[0].Jobs {
		if job.IsStatusInProgress() {
			return true
		}
	}
	return false
}

func (m *model) shouldShowSteps() bool {
	loadingSteps := false
	if m.flat {
		check := m.checksList.SelectedItem()
		if check != nil {
			ci, ok := check.(*checkItem)
			if ok {
				loadingSteps = ci.loadingSteps
			}
		}
	} else {
		job := m.jobsList.SelectedItem()
		if job != nil {
			ji, ok := job.(*jobItem)
			if ok {
				loadingSteps = ji.loadingSteps
			}
		}
	}

	return loadingSteps || (len(m.stepsList.Items()) > 0)
}

func (m *model) viewLogs() string {
	title := "Job Logs"
	w := m.logsWidth()
	h := m.getMainContentHeight()
	if m.focusedPane == PaneLogs {
		title = makePill(title, m.styles.focusedPaneTitleStyle, m.styles.colors.focusedColor)
		s := m.styles.focusedPaneTitleBarStyle.MarginBottom(0)
		title = s.Render(title)
	} else {
		title = makePill(title, m.styles.unfocusedPaneTitleStyle, m.styles.colors.unfocusedColor)
		s := m.styles.unfocusedPaneTitleBarStyle.MarginBottom(0)
		title = s.Render(title)
	}

	if m.logsInput.Value() != "" && !m.logsInput.Focused() {
		matches := fmt.Sprintf("%d matches", m.numHighlights)
		if m.numHighlights == 0 {
			matches = "no matches"
		}
		title = lipgloss.JoinHorizontal(lipgloss.Top, title, " ",
			m.styles.faintFgStyle.Render(matches))
	}

	inputView := ""
	ji := m.getSelectedJobItem()
	if ji != nil && m.logsViewport.GetContent() != "" && ji.logsStderr == "" {
		inputView = lipgloss.NewStyle().
			Width(w).
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(
				m.styles.colors.fainterColor).
			Render(m.logsInput.View())
	}

	return lipgloss.NewStyle().
		Height(h).
		MaxHeight(h).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, inputView, m.logsContentView()))
}

func (m *model) setFocusedPaneStyles() {
	switch m.focusedPane {
	case PaneChecks:
		m.checksDelegate.(*checksDelegate).focused = true
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListFocusedStyles(&m.checksList, &m.checksDelegate, PaneChecks)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneRuns:
		m.runsDelegate.(*runsDelegate).focused = true
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListFocusedStyles(&m.runsList, &m.runsDelegate, PaneRuns)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneJobs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = true
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListFocusedStyles(&m.jobsList, &m.jobsDelegate, PaneJobs)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneSteps:
		m.checksDelegate.(*checksDelegate).focused = false
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = true
		m.setListUnfocusedStyles(&m.checksList, &m.checksDelegate)
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListFocusedStyles(&m.stepsList, &m.stepsDelegate, PaneSteps)
	case PaneLogs:
		m.checksDelegate.(*checksDelegate).focused = false
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListUnfocusedStyles(&m.checksList, &m.checksDelegate)
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	}

	w := m.logsWidth()
	m.logsViewport.SetWidth(w)
	m.logsInput.SetWidth(int(math.Max(float64(0), float64(
		w-lipgloss.Width(m.logsInput.Prompt)-2))))
}

func (m *model) setListFocusedStyles(l *list.Model, delegate *list.ItemDelegate, p pane) {
	if m.width != 0 && m.width <= m.smallScreenWidth {
		l.Styles.Title = m.styles.focusedPaneTitleStyle.Bold(false)
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle.Bold(false)
		l.Title = m.getPaneTitle(l)
	} else {
		l.Styles.Title = m.styles.focusedPaneTitleStyle
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
		l.Title = makePill(m.getPaneTitle(l), l.Styles.Title, m.styles.colors.focusedColor)
	}

	w := m.getFocusedPaneWidth(l, p)
	l.SetWidth(w)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(w)
	l.SetDelegate(*delegate)
}

func (m *model) setListUnfocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	if m.width > m.smallScreenWidth {
		l.Styles.Title = m.styles.unfocusedPaneTitleStyle
		l.Title = makePill(m.getPaneTitle(l), l.Styles.Title, m.styles.colors.unfocusedColor)
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
	} else {
		l.Styles.Title = m.styles.unfocusedPaneTitleStyle.Bold(false)
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle.Bold(false)
	}

	w := m.getUnfocusedPaneWidth()
	l.SetWidth(w)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(w)
	l.SetDelegate(*delegate)
}

func newRunsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newRunItemDelegate(styles)
	return newList(styles, d), d
}

func newJobsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newJobItemDelegate(styles)
	return newList(styles, d), d
}

func newStepsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newStepItemDelegate(styles)
	return newList(styles, d), d
}

func newChecksDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newCheckItemDelegate(styles)
	return newList(styles, d), d
}

func newList(styles styles, delegate list.ItemDelegate) list.Model {
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.KeyMap.Quit = quitKey
	l.Paginator.Type = paginator.Arabic
	l.Styles.StatusBar = l.Styles.StatusBar.Foreground(styles.colors.faintColor)
	l.Styles.StatusEmpty = l.Styles.StatusEmpty.Foreground(styles.colors.faintColor)
	l.Styles.StatusBarActiveFilter = l.Styles.StatusBarActiveFilter.Foreground(
		styles.colors.faintColor,
	)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.Foreground(
		styles.colors.faintColor,
	)
	l.Styles.NoItems = l.Styles.NoItems.Width(defaultUnfocusedLargePaneWidth).
		Foreground(styles.colors.faintColor)
	l.Styles.PaginationStyle = lipgloss.NewStyle().
		Foreground(styles.colors.faintColor).
		MarginLeft(1).
		MarginBottom(1)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1)
	l.SetSpinner(spinner.Dot)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.StartSpinner()
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return l
}

func (m *model) updateListsSpinners() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if m.flat {
		cCmds := m.updateFlatModeSpinners()
		cmds = append(cmds, cCmds...)
	} else {
		rCmds := m.updateRunsModeSpinners()
		cmds = append(cmds, rCmds...)
	}

	return cmds
}

func (m *model) updateFlatModeSpinners() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	if len(m.checksList.VisibleItems()) == 0 {
		return cmds
	}

	check := m.checksList.SelectedItem()
	if check == nil {
		return cmds
	}
	ci, ok := check.(*checkItem)
	if !ok {
		return cmds
	}

	if ci.loadingSteps {
		cmds = append(cmds, m.stepsList.StartSpinner())
	} else {
		m.stepsList.StopSpinner()
	}

	return cmds
}

func (m *model) updateRunsModeSpinners() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	if len(m.runsList.VisibleItems()) == 0 {
		return cmds
	}

	run := m.runsList.SelectedItem()
	if run == nil {
		return cmds
	}
	ri, ok := run.(*runItem)
	if !ok {
		return cmds
	}

	if ri.loadingSteps {
		cmds = append(cmds, m.stepsList.StartSpinner())
	} else {
		m.stepsList.StopSpinner()
	}

	if ri.loadingJobs {
		cmds = append(cmds, m.jobsList.StartSpinner())
	} else {
		m.jobsList.StopSpinner()
	}

	return cmds
}

func (m *model) setJobsListItemsFromRun(ri *runItem) tea.Cmd {
	if ri == nil {
		return nil
	}

	jobs := make([]list.Item, 0)
	for _, ji := range ri.jobsItems {
		jobs = append(jobs, ji)
	}

	log.Info("updateJobsList", "setting items - len(jobs)", len(jobs))
	return m.jobsList.SetItems(jobs)
}

func (m *model) setStepsListItemsFromCheck(ci *checkItem) tea.Cmd {
	if ci == nil {
		return nil
	}
	log.Debug("updating steps list from check", "check", ci.job.Name)

	steps := make([]list.Item, 0)
	for _, ji := range ci.steps {
		steps = append(steps, ji)
	}

	log.Info("updateStepsList", "setting items - len(steps)", len(steps))
	return m.stepsList.SetItems(steps)
}

func (m *model) setStepsListItemsFromJob(ji *jobItem) tea.Cmd {
	if ji == nil {
		return nil
	}

	steps := make([]list.Item, 0)
	for _, ji := range ji.steps {
		steps = append(steps, ji)
	}

	log.Info("updateStepsList", "setting items - len(steps)", len(steps))
	return m.stepsList.SetItems(steps)
}

func (m *model) getSelectedCheckItem() *checkItem {
	check := m.checksList.SelectedItem()
	if check == nil {
		return nil
	}
	ci, ok := check.(*checkItem)
	if !ok {
		return nil
	}

	return ci
}

func (m *model) getSelectedRunItem() *runItem {
	run := m.runsList.SelectedItem()
	if run == nil {
		return nil
	}
	ri, ok := run.(*runItem)
	if !ok {
		return nil
	}

	return ri
}

func (m *model) getSelectedJobItem() *jobItem {
	if m.flat {
		check := m.checksList.SelectedItem()
		if check == nil {
			return nil
		}
		ci, ok := check.(*checkItem)
		if !ok {
			return nil
		}
		return &ci.jobItem
	} else {
		job := m.jobsList.SelectedItem()
		if job == nil {
			// this can happen because filtering is done asyncly
			// In that case m.jobsList.VisibleItems() returns 0 items
			return nil
		}
		ji, ok := job.(*jobItem)
		if !ok {
			return nil
		}
		return ji
	}
}

func (m *model) getSelectedStepItem() *stepItem {
	step := m.stepsList.SelectedItem()
	if step == nil {
		return nil
	}
	si, ok := step.(*stepItem)
	if !ok {
		return nil
	}

	return si
}

func (m *model) logsWidth() int {
	if m.width == 0 {
		return 0
	}

	if m.zoomedPane != nil && *m.zoomedPane == PaneLogs {
		return m.width - 1
	}

	var borders int
	if m.width != 0 && m.width <= m.smallScreenWidth {
		borders = 1
	} else if m.flat {
		borders = 1
	} else {
		borders = 2
	}

	scbar := 0
	ji := m.getSelectedJobItem()
	if ji != nil && (len(ji.renderedLogs) > 0 || len(ji.renderedText) > 0) &&
		m.isScrollbarVisible() {
		scbar = lipgloss.Width(m.scrollbar.(scrollbar.Vertical).View())
	}

	steps := 0
	if m.shouldShowSteps() {
		steps = m.stepsList.Width()
		borders = borders + 1
	}

	w := m.width - steps - borders - scbar
	if m.flat {
		w -= m.checksList.Width()
	} else {
		w -= m.runsList.Width() + m.jobsList.Width()
	}
	return w
}

func (m *model) loadingLogsView() string {
	return m.fullScreenMessageView(
		lipgloss.JoinVertical(lipgloss.Left, m.logsSpinner.View()))
}

func (m *model) fullScreenMessageView(message string) string {
	return lipgloss.Place(
		m.logsWidth(),
		m.getLogsViewportHeight()-1,
		lipgloss.Center,
		0.75,
		message,
	)
}

func (m *model) noLogsView(message string) string {
	emptySetArt := strings.Builder{}
	for _, char := range art.EmptySet {
		if char == '╱' {
			emptySetArt.WriteString(lipgloss.NewStyle().Foreground(
				m.styles.colors.errorColor).Render("╱"))
		} else {
			emptySetArt.WriteString(m.styles.watermarkIllustrationStyle.Render(string(char)))
		}
	}

	return m.fullScreenMessageView(
		lipgloss.JoinVertical(
			lipgloss.Center,
			emptySetArt.String(),
			m.styles.noLogsStyle.Render(message),
		),
	)
}

func (m *model) isScrollbarVisible() bool {
	return m.logsViewport.TotalLineCount() > m.logsViewport.VisibleLineCount()
}

func (m *model) enrichRunWithJobsStepsV2(msg workflowRunStepsFetchedMsg) []tea.Cmd {
	log.Debug("enriching run with all jobs steps", "runId", msg.runId)
	cmds := make([]tea.Cmd, 0)
	jobsMap := make(map[string]api.CheckRunWithSteps)
	checks := msg.data.Resource.WorkflowRun.CheckSuite.CheckRuns.Nodes
	for _, check := range checks {
		jobsMap[fmt.Sprint(check.DatabaseId)] = check
	}

	ri := m.getRunItemById(msg.runId)
	if ri == nil {
		log.Error("run not found when trying to enrich with steps", "msg.runId", msg.runId)
		return cmds
	}

	ri.loadingSteps = false
	for _, ji := range ri.jobsItems {
		ji.loadingSteps = false
		jobWithSteps, ok := jobsMap[ji.job.Id]
		if !ok {
			continue
		}

		steps := make([]*stepItem, 0)
		for _, step := range jobWithSteps.Steps.Nodes {
			si := NewStepItem(step, jobWithSteps.Url, m.styles)

			steps = append(steps, &si)
		}

		ji.steps = steps
		if sji := m.getSelectedJobItem(); areJobItemsEqual(sji, ji) && sji != nil &&
			len(sji.steps) == 0 {
			m.setStepsListItemsFromJob(ji)
		}
	}

	if areRunItemsEqual(m.getSelectedRunItem(), ri) {
		cmds = append(cmds, m.updateCurrentRunJobsListItems()...)
		cmds = append(cmds, m.updateCurrentJobStepsListItems()...)
	}

	return cmds
}

func (m *model) enrichCheckWithSteps(msg checkStepsFetchedMsg) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ci := m.getCheckItemById(msg.checkId)
	if ci == nil {
		log.Error("check not found when trying to enrich with steps", "msg.checkId", msg.checkId)
		return nil
	}

	log.Debug(
		"enriching check with steps",
		"msg.checkId",
		msg.checkId,
		"ci.jobId",
		ci.job.Id,
		"name",
		ci.job.Name,
	)
	ci.loadingSteps = false

	steps := make([]*stepItem, 0)
	for _, step := range msg.steps {
		si := NewStepItem(step, ci.job.Link, m.styles)
		steps = append(steps, &si)
	}

	ci.steps = steps

	if areCheckItemsEqual(m.getSelectedCheckItem(), ci) {
		cmds = append(cmds, m.updateCurrentCheckStepsListItems()...)
	}
	return cmds
}

func (m *model) enrichRunWithJobs(msg runJobsFetchedMsg) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getRunItemById(msg.runId)
	if ri == nil {
		log.Error("run not found when trying to enrich with jobs", "msg", msg)
		return nil
	}

	ri.loadingJobs = false
	if msg.err != nil {
		log.Error("runJobsFetchedMsg", "err", msg.err)
		return nil
	}

	jobs := make([]*jobItem, 0)
	for _, job := range msg.jobs {
		ji := NewJobItem(job, m.styles)
		jobs = append(jobs, &ji)
	}

	log.Info("enriching run with jobs", "runId", ri.run.Id, "len(jobs)", len(jobs))
	ri.run.Jobs = msg.jobs
	ri.jobsItems = jobs

	if areRunItemsEqual(m.getSelectedRunItem(), ri) {
		cmds = append(cmds, m.updateCurrentRunJobsListItems()...)
	}
	return cmds
}

func (m *model) onCheckChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currCheck := m.getSelectedCheckItem()
	if currCheck == nil {
		log.Error("check changed but current check is nil")
		return nil
	}

	if currCheck.hasInProgressSteps() || currCheck.loadingSteps {
		cmds = append(cmds, m.makeFetchCheckStepsCmd(currCheck.job.Id))
	}

	if !currCheck.initiatedLogsFetch && !currCheck.isStatusInProgress() {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	}

	cmds = append(cmds, m.setStepsListItemsFromCheck(currCheck))

	return cmds
}

func (m *model) onRunChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.jobsList.ResetSelected()
	m.jobsList.ResetFilter()
	ri := m.getSelectedRunItem()
	if ri == nil {
		log.Error("run changed but there is no run")
		return cmds
	}

	if !ri.loadingSteps &&
		(ri.lastFetchSteps.IsZero() || time.Since(ri.lastFetchSteps) > refreshInterval) {
		ri.loadingSteps = true
		ri.lastFetchSteps = time.Now()
		cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
	}
	if ri.ShouldFetchJobs() {
		log.Info("run changed - fetching jobs", "runId", ri.run.Id)
		ri.loadingJobs = true
		ri.lastFetchJobs = time.Now()
		cmds = append(cmds, m.jobsList.StartSpinner())
		cmds = append(cmds, m.makeFetchWorkflowRunJobsCmd(*ri.run))
	}

	m.setJobsListItemsFromRun(m.getSelectedRunItem())

	cmds = append(cmds, m.updateListsSpinners()...)
	cmds = append(cmds, m.onJobChanged()...)

	m.logsViewport.GotoTop()

	return cmds
}

func (m *model) onJobChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currJob := m.getSelectedJobItem()

	if currJob != nil && !currJob.initiatedLogsFetch && !currJob.isStatusInProgress() {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	} else if currJob == nil {
		log.Error("job changed but current job is nil")
	}

	cmds = append(cmds, m.renderJobLogs())
	m.goToErrorInLogs()

	cmds = append(cmds, m.setStepsListItemsFromJob(currJob))

	return cmds
}

func (m *model) onStepChanged() {
	ji := m.getSelectedJobItem()
	step := m.stepsList.SelectedItem()
	cursor := m.stepsList.Cursor()

	if ji == nil || step == nil {
		return
	}

	if cursor == len(m.stepsList.Items())-1 {
		m.logsViewport.GotoBottom()
		return
	}

	for i, log := range ji.logs {
		if log.Time.After(step.(*stepItem).step.StartedAt) {
			m.logsViewport.SetYOffset(i - 1)
			return
		}
	}
}

func (m *model) renderJobLogs() tea.Cmd {
	ji := m.getSelectedJobItem()
	if ji == nil || ji.loadingLogs {
		m.logsViewport.SetContent("")
	}

	if ji == nil {
		return nil
	}

	if ji.isStatusInProgress() {
		return m.inProgressSpinner.Tick
	}

	if ji.logsErr != nil {
		m.logsViewport.SetContent(ji.logsStderr)
		m.setHeights()

		return nil
	}

	if len(ji.renderedLogs) != 0 {
		m.logsViewport.SetContentLines(ji.renderedLogs)
		m.setHeights()

		return nil
	}

	if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun ||
		ji.job.Kind == data.JobKindExternal {
		m.logsViewport.SetContent(ji.renderedText)
		m.logsViewport.SetWidth(5)
		m.setHeights()

		return nil
	}

	ji.renderedLogs, ji.unstyledLogs = m.renderLogs(ji)
	m.logsViewport.SetContentLines(ji.renderedLogs)
	m.setHeights()

	return nil
}

func (m *model) logsContentView() string {
	if m.prWithChecks.Number != 0 && len(m.prWithChecks.Commits.Nodes) > 0 &&
		m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.CheckRunCount == 0 {
		return m.fullScreenMessageView(
			lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(m.styles.tint.BrightWhite).Render(art.CheckmarkSign),
				"",
				m.styles.faintFgStyle.Bold(true).Render(
					fmt.Sprintf("No checks reported on the '%s' branch", m.prWithChecks.HeadRefName),
				),
			))
	}

	ri := m.getSelectedRunItem()
	if ri != nil && ri.run.Bucket == data.CheckBucketActionRequired {
		return m.fullScreenMessageView(lipgloss.JoinVertical(
			lipgloss.Center,
			m.styles.faintFgStyle.Render(art.StopSign),
			m.styles.faintFgStyle.Bold(true).
				Render("This workflow is awaiting approval from a maintainer"),
		))
	}

	ji := m.getSelectedJobItem()
	if ji == nil {
		return m.fullScreenMessageView(
			m.styles.faintFgStyle.Bold(true).Render("Nothing selected..."),
		)
	}

	if ji.job.Conclusion == api.ConclusionSkipped {
		return m.noLogsView("This job was skipped")
	}

	if ji.isStatusInProgress() {
		text := ""
		if ji.job.State == api.StatusWaiting && ji.job.PendingEnv != "" {
			text = lipgloss.NewStyle().Foreground(
				m.styles.colors.warnColor).Render("Waiting for review: " + ji.job.PendingEnv +
				" needs approval to start deploying changes.")
		} else {
			text = "This job is still in progress"
		}

		return m.fullScreenMessageView(
			m.renderFullScreenLogsSpinner(text, "view the job on github.com"),
		)
	}

	if ji.loadingLogs || (ji.loadingSteps && len(ji.steps) == 0) {
		return m.loadingLogsView()
	}

	if ji.job.Bucket == data.CheckBucketCancel {
		return m.fullScreenMessageView(lipgloss.JoinVertical(lipgloss.Center,
			m.styles.faintFgStyle.Render(art.StopSign),
			m.styles.faintFgStyle.Bold(true).Render("This job was cancelled")))
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "HTTP 410:") {
		return m.fullScreenMessageView(
			"The logs for this run have expired and are no longer available.",
		)
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "is still in progress;") {
		return m.fullScreenMessageView(m.renderFullScreenLogsSpinner(
			"This run is still in progress", "view the run on github.com"))
	}

	if m.isScrollbarVisible() {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			m.logsViewport.View(),
			m.scrollbar.(scrollbar.Vertical).View(),
		)
	}
	return m.logsViewport.View()
}

func (m *model) getRunItemById(runId string) *runItem {
	for _, run := range m.runsList.Items() {
		ri := run.(*runItem)
		if ri.run.Id == runId {
			return ri
		}
	}
	return nil
}

func (m *model) getCheckItemById(checkId string) *checkItem {
	for _, check := range m.checksList.Items() {
		ci := check.(*checkItem)
		if ci.job.Id == checkId {
			return ci
		}
	}
	return nil
}

func (m *model) getJobItemById(jobId string) *jobItem {
	if m.flat {
		for _, check := range m.checksList.Items() {
			ci := check.(*checkItem)
			if ci.job.Id == jobId {
				return &ci.jobItem
			}
		}
	} else {
		for _, run := range m.runsList.Items() {
			ri := run.(*runItem)
			for i := range ri.jobsItems {
				if ri.jobsItems[i].job.Id == jobId {
					return ri.jobsItems[i]
				}
			}
		}
	}
	return nil
}

func (m *model) renderLogs(ji *jobItem) ([]string, []string) {
	defer utils.TimeTrack(time.Now(), "rendering logs")
	w := m.logsViewport.Width() - m.styles.scrollbarStyle.GetWidth()
	expand := ExpandSymbol + " "
	lines := make([]string, 0)
	unstyledLines := make([]string, 0)
	for i, log := range ji.logs {
		rendered := log.Log
		unstyled := ansi.Strip(log.Log)
		switch log.Kind {
		case data.LogKindError:
			ji.errorLine = i
			rendered = strings.Replace(rendered, parser.ErrorMarker, "", 1)
			unstyled = rendered
			rendered = m.styles.errorBgStyle.Width(w).Render(lipgloss.JoinHorizontal(lipgloss.Top,
				m.styles.errorTitleStyle.Render("Error: "), m.styles.errorStyle.Render(rendered)))
		case data.LogKindCommand:
			rendered = strings.Replace(rendered, parser.CommandMarker, "", 1)
			unstyled = rendered
			rendered = m.styles.commandStyle.Render(rendered)
		case data.LogKindGroupStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			unstyled = rendered
			rendered = m.styles.groupStartMarkerStyle.Render(rendered)
		case data.LogKindJobCleanup:
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			unstyled = rendered
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepNone:
			sep := ""
			unstyledSep := ""
			if log.Depth > 0 {
				dm := strings.Repeat(
					fmt.Sprintf("%s  ", Separator), log.Depth)
				unstyledSep = dm
				sep = m.styles.separatorStyle.Render(dm)
			}
			unstyled = unstyledSep + unstyled
			rendered = sep + rendered
		}
		lines = append(lines, rendered)
		unstyledLines = append(unstyledLines, unstyled)
	}
	return lines, unstyledLines
}

func (m *model) getFocusedPaneWidth(l *list.Model, p pane) int {
	if m.zoomedPane != nil && p == *m.zoomedPane {
		return m.width - 1
	}
	if m.width > m.smallScreenWidth {
		if len(l.Items()) == 0 {
			return m.unfocusedLargePaneWidth
		}
		return m.focusedLargePaneWidth
	}

	return focusedSmallPaneWidth
}

func (m *model) getPaneTitle(l *list.Model) string {
	if m.width != 0 && m.width <= m.smallScreenWidth {
		s := m.styles.focusedPaneTitleStyle.Bold(false).UnsetBackground()
		switch m.focusedPane {
		case PaneChecks:
			return lipgloss.JoinHorizontal(lipgloss.Top,
				makePill(s.Bold(true).Render("Checks"), l.Styles.Title,
					m.styles.colors.focusedColor), s.Render(" > Steps"))
		case PaneRuns:
			return lipgloss.JoinHorizontal(lipgloss.Top,
				makePill(s.Bold(true).Render("Runs"), l.Styles.Title,
					m.styles.colors.focusedColor), s.Render(" > Jobs > Steps"))
		case PaneJobs:
			return lipgloss.JoinHorizontal(lipgloss.Top, s.Render("Runs > "),
				makePill(s.Bold(true).Render("Jobs"), l.Styles.Title,
					m.styles.colors.focusedColor), s.Render(" > Steps"))
		case PaneSteps:
			if m.flat {
				return lipgloss.JoinHorizontal(
					lipgloss.Top,
					s.Render("Checks > "),
					makePill(
						s.Bold(true).Render("Steps"),
						l.Styles.Title,
						m.styles.colors.focusedColor,
					),
				)
			}
			return lipgloss.JoinHorizontal(
				lipgloss.Top,
				s.Render("Runs > Jobs > "),
				makePill(
					s.Bold(true).Render("Steps"),
					l.Styles.Title,
					m.styles.colors.focusedColor,
				),
			)
		case PaneLogs:
			return ""
		}
	}

	_, itemsName := l.StatusBarItemName()
	return strings.ToUpper(string(itemsName[0])) + itemsName[1:]
}

func (m *model) getUnfocusedPaneWidth() int {
	if m.width != 0 && m.width <= m.smallScreenWidth {
		return 0
	}

	return m.unfocusedLargePaneWidth
}

func (m *model) goToErrorInLogs() {
	currJob := m.getSelectedJobItem()
	if currJob == nil {
		return
	}

	if currJob.errorLine > 0 {
		for i, step := range m.stepsList.VisibleItems() {
			if api.IsFailureConclusion(step.(*stepItem).step.Conclusion) {
				m.stepsList.Select(i)
				break
			}
		}
		m.logsViewport.SetYOffset(currJob.errorLine)
	} else {
		m.logsViewport.GotoTop()
	}
}

func (m *model) getLogsViewportHeight() int {
	h := m.getMainContentHeight()

	// TODO: take borders from logsInput view
	vph := h - paneTitleHeight
	if m.logsViewport.GetContent() != "" {
		vph -= lipgloss.Height(m.logsInput.View()) + 2 // borders
	}
	m.logsViewport.SetHeight(vph)
	m.scrollbar, _ = m.scrollbar.Update(scrollbar.HeightMsg(vph))

	return vph
}

func (m *model) getMainContentHeight() int {
	return m.height - headerHeight - footerHeight
}

func (m *model) setHeights() {
	h := m.getMainContentHeight()

	m.checksList.SetHeight(h)
	m.runsList.SetHeight(h)
	m.jobsList.SetHeight(h)
	m.stepsList.SetHeight(h)

	lh := m.getLogsViewportHeight()
	m.logsViewport.SetHeight(lh)
	m.scrollbar, _ = m.scrollbar.Update(scrollbar.HeightMsg(lh))
}

func (m *model) setWidths() {
	m.help.SetWidth(m.width)
	w := m.logsWidth()
	m.logsViewport.SetWidth(w)
	m.logsInput.SetWidth(w - 10)
}

func (m *model) renderFullScreenLogsSpinner(message string, cta string) string {
	return lipgloss.JoinVertical(
		lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Center,
			m.inProgressSpinner.View(),
			"  ",
			lipgloss.NewStyle().Foreground(m.styles.colors.warnColor).Render(message)),
		"",
		m.styles.faintFgStyle.Render("(logs will be available when it is complete)"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, m.styles.faintFgStyle.Render("Press "),
			m.styles.keyStyle.Render("o"),
			m.styles.faintFgStyle.Render(" to "),
			m.styles.faintFgStyle.Render(cta)),
	)
}

func (m *model) onWorkflowRunsFetched() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if m.flat {
		prevCheck := m.getSelectedCheckItem()

		cmds = append(cmds, m.updateChecksListItems()...)

		if prevCheck == nil && len(m.checksList.Items()) > 0 {
			cmds = append(cmds, m.onCheckChanged()...)
		} else if len(m.checksList.Items()) > 0 {
			currCheck := m.getSelectedCheckItem()
			if currCheck != nil && currCheck.hasInProgressSteps() {
				cmds = append(cmds, m.makeFetchCheckStepsCmd(currCheck.job.Id))
			}
		}

		if prevCheck != nil && !prevCheck.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}

		if !areCheckItemsEqual(prevCheck, m.getSelectedCheckItem()) {
			cmds = append(cmds, m.onCheckChanged()...)
		}
	} else {
		prevRun := m.getSelectedRunItem()
		cmds = append(cmds, m.updateRunsListItems()...)

		if len(m.runsList.Items()) > 0 {
			ri := m.getSelectedRunItem()
			if ri != nil && ri.ShouldFetchJobs() {
				ri.loadingJobs = true
				ri.lastFetchJobs = time.Now()
				log.Info(
					"workflow runs fetched - fetching the current run's jobs",
					"runId",
					ri.run.Id,
				)
				cmds = append(cmds, m.makeFetchWorkflowRunJobsCmd(*ri.run))
			}

			if ri != nil && ri.run != nil && !ri.loadingSteps &&
				(ri.lastFetchSteps.IsZero() || time.Since(ri.lastFetchSteps) > refreshInterval) {
				ri.loadingSteps = true
				ri.lastFetchSteps = time.Now()
				cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
				if !areRunItemsEqual(prevRun, ri) {
					cmds = append(cmds, m.onRunChanged()...)
				}
			}
		}

		currJob := m.getSelectedJobItem()
		if currJob != nil && !currJob.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}

		if !areRunItemsEqual(prevRun, m.getSelectedRunItem()) {
			cmds = append(cmds, m.onRunChanged()...)
		}
	}

	cmds = append(cmds, m.updateListsSpinners()...)

	return cmds
}

func (m *model) updateChecksListItems() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	sorted := make([]data.WorkflowJob, 0)
	for _, run := range m.workflowRuns {
		sorted = append(sorted, run.Jobs...)
	}
	data.SortJobs(sorted)

	checksMap := m.makeCurrentChecksMap()
	for i, job := range sorted {
		ci := NewCheckItem(job, m.styles)

		if existing, ok := checksMap[job.Id]; ok {
			newJobData := ci.job
			ci.jobItem = existing.jobItem
			ci.job = newJobData
		} else {
			cmds = append(cmds, m.checksList.InsertItem(i, &ci))
		}
	}

	return cmds
}

func (m *model) updateRunsListItems() []tea.Cmd {
	prevRun := m.getSelectedRunItem()
	cmds := make([]tea.Cmd, 0)
	for i, run := range m.workflowRuns {
		ri := m.getRunItemById(run.Id)
		if ri == nil {
			nr := NewRunItem(run, m.styles)
			ri = &nr

			cmds = append(cmds, m.runsList.InsertItem(i, ri))
		} else {
			ri.run = &run
		}

		jobs := make([]*jobItem, 0)
		cmds = append(cmds, m.inProgressSpinner.Tick)
		for _, job := range run.Jobs {
			ji := m.getJobItemById(job.Id)
			if ji == nil {
				nji := NewJobItem(job, m.styles)
				ji = &nji
			}
			ji.job = &job
			jobs = append(jobs, ji)
		}

		ri.jobsItems = jobs
	}

	if areRunItemsEqual(m.getSelectedRunItem(), prevRun) {
		cmds = append(cmds, m.updateCurrentRunJobsListItems()...)
	}

	return cmds
}

func (m *model) viewCommitStatus(bgStyle lipgloss.Style) string {
	if len(m.pr.Commits.Nodes) == 0 {
		return ""
	}

	s := bgStyle.Height(2).MaxHeight(2)
	status := m.pr.Commits.Nodes[0].Commit.StatusCheckRollup.State
	res := ""
	switch status {
	case api.CommitStateSuccess:
		res = s.Foreground(m.styles.colors.successColor).Render(SuccessIcon)
	case api.CommitStateError, api.CommitStateFailure:
		res = s.Foreground(m.styles.colors.errorColor).Render(FailureIcon)
	case api.CommitStateExpected, api.CommitStatePending:
		res = s.Foreground(m.styles.colors.warnColor).Render(WaitingIcon)
	}

	if res != "" {
		return bgStyle.Padding(0, 1).
			BorderForeground(m.styles.colors.darkColor).
			BorderBackground(bgStyle.GetBackground()).
			Border(
				lipgloss.ThickBorder(), false, true, false, false).
			Render(res)
	}

	return string(status)
}

func (m *model) paneStyle(pane pane) lipgloss.Style {
	// the border of the pane is the actually rendered by the previous pane
	prev := m.previousPane()
	if prev != m.focusedPane && prev == pane {
		return m.styles.focusedPaneStyle
	}

	return m.styles.paneStyle
}

func (m *model) stopSpinners() {
	m.checksList.StopSpinner()
	m.runsList.StopSpinner()
	m.jobsList.StopSpinner()
}

func (m *model) resetStepsState() {
	m.logsViewport.ClearHighlights()
	m.numHighlights = 0
	m.logsInput.Reset()
	m.stepsList.ResetSelected()
	m.stepsList.ResetFilter()
}

type mode int

const (
	ModeRun mode = iota
	ModePR
	ModeRepo
)

func (m *model) mode() mode {
	if m.runID != "" {
		return ModeRun
	} else if m.prNumber != "" {
		return ModePR
	}

	return ModeRepo
}

var cachedSpinner spinner.Model

func (m *model) enrichRepoModeFetchedRunsWithExistingJobs(
	msg repoModeRunsFetchedMsg,
) repoModeRunsFetchedMsg {
	if msg.Err != nil {
		return msg
	}

	existingMap := make(map[string]*runItem)
	for _, item := range m.runsList.Items() {
		ri := item.(*runItem)
		existingMap[ri.run.Id] = ri
	}

	enriched := make([]data.WorkflowRun, len(msg.Runs))

	for i, run := range msg.Runs {
		if existing, ok := existingMap[run.Id]; ok && len(existing.run.Jobs) > 0 {
			run.Jobs = existing.run.Jobs
			log.Debug(
				"enriched run with existing jobs",
				"runId",
				run.Id,
				"len(jobs)",
				len(existing.run.Jobs),
			)
		}
		enriched[i] = run
	}
	return repoModeRunsFetchedMsg{
		Repo: msg.Repo,
		Runs: enriched,
	}
}

func (m *model) updateListsFiltersStates() {
	if len(m.checksList.VisibleItems()) > 0 || m.checksList.FilterState() == list.FilterApplied {
		m.checksList.SetShowStatusBar(true)
	} else {
		m.checksList.SetShowStatusBar(false)
	}

	if len(m.runsList.VisibleItems()) > 0 || m.runsList.FilterState() == list.FilterApplied {
		m.runsList.SetShowStatusBar(true)
	} else {
		m.runsList.SetShowStatusBar(false)
	}

	if len(m.jobsList.VisibleItems()) > 0 || m.jobsList.FilterState() == list.FilterApplied {
		m.jobsList.SetShowStatusBar(true)
	} else {
		m.jobsList.SetShowStatusBar(false)
	}

	if len(m.stepsList.VisibleItems()) > 0 || m.stepsList.FilterState() == list.FilterApplied {
		m.stepsList.SetShowStatusBar(true)
	} else {
		m.stepsList.SetShowStatusBar(false)
	}
}

func (m *model) selectRunItem(item *runItem) {
	if item == nil {
		return
	}
	for i, ri := range m.runsList.VisibleItems() {
		ri := ri.(*runItem)
		if areRunItemsEqual(item, ri) {
			m.runsList.Select(i)
			break
		}
	}
}

func (m *model) selectJobItem(item *jobItem) {
	if item == nil {
		return
	}
	for i, ji := range m.jobsList.VisibleItems() {
		ji := ji.(*jobItem)
		if areJobItemsEqual(item, ji) {
			m.jobsList.Select(i)
			break
		}
	}
}

func (m *model) selectStepItem(item *stepItem) {
	if item == nil {
		return
	}
	for i, si := range m.stepsList.VisibleItems() {
		si := si.(*stepItem)
		if areStepItemsEqual(item, si) {
			m.stepsList.Select(i)
			break
		}
	}
}

func (m *model) selectCheckItem(item *checkItem) {
	if item == nil {
		return
	}
	for i, ci := range m.checksList.VisibleItems() {
		ci := ci.(*checkItem)
		if areCheckItemsEqual(item, ci) {
			m.checksList.Select(i)
			break
		}
	}
}

func (m *model) makeCurrentChecksMap() map[string]*checkItem {
	checks := make(map[string]*checkItem)
	for _, ci := range m.checksList.Items() {
		if ci, ok := ci.(*checkItem); ok {
			checks[ci.job.Id] = ci
		}
	}
	return checks
}

func (m *model) makeCurrentJobsMap() map[string]*jobItem {
	jobs := make(map[string]*jobItem)
	for _, ji := range m.jobsList.Items() {
		if ji, ok := ji.(*jobItem); ok {
			jobs[ji.job.Id] = ji
		}
	}
	return jobs
}

func (m *model) makeCurrentStepsMap() map[string]*stepItem {
	steps := make(map[string]*stepItem)
	for _, ji := range m.stepsList.Items() {
		if si, ok := ji.(*stepItem); ok {
			steps[si.step.Name] = si
		}
	}
	return steps
}

func (m *model) updateCurrentRunJobsListItems() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getSelectedRunItem()
	if ri == nil {
		return nil
	}

	jm := m.makeCurrentJobsMap()
	for i, ji := range ri.jobsItems {
		if existing, ok := jm[ji.job.Id]; ok {
			existing.job = ji.job
		} else {
			cmds = append(cmds, m.jobsList.InsertItem(i, ji))
		}
	}
	return cmds
}

func (m *model) updateCurrentCheckStepsListItems() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ci := m.getSelectedCheckItem()
	if ci == nil {
		return nil
	}

	sm := m.makeCurrentStepsMap()
	for i, si := range ci.steps {
		if existing, ok := sm[si.step.Name]; ok {
			existing.step = si.step
		} else {
			cmds = append(cmds, m.stepsList.InsertItem(i, si))
		}
	}
	return cmds
}

func (m *model) updateCurrentJobStepsListItems() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ji := m.getSelectedJobItem()
	if ji == nil {
		return nil
	}
	log.Debug("updating current job steps", "job", ji.job.Id, "len", len(ji.steps))

	sm := m.makeCurrentStepsMap()

	for i, si := range ji.steps {
		if existing, ok := sm[si.step.Name]; ok {
			existing.step = si.step
		} else {
			cmds = append(cmds, m.stepsList.InsertItem(i, si))
		}
	}
	return cmds
}

func (m *model) isInProgress() bool {
	isInProgress := false
	if m.mode() == ModePR {
		isInProgress = m.prWithChecks.Number != 0 && m.prWithChecks.IsStatusCheckInProgress()
	}
	if isInProgress {
		return true
	}

	if m.flat {
		for _, ci := range m.checksList.Items() {
			if ci, ok := ci.(*checkItem); ok && ci.isStatusInProgress() {
				return true
			}
		}
	}

	for _, ri := range m.runsList.Items() {
		if ri, ok := ri.(*runItem); ok && ri.HasNotConcluded() {
			return true
		}
	}

	return false
}
