package tui

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/paginator"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/textinput"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log/v2"
	"github.com/charmbracelet/x/ansi"
	checks "github.com/dlvhdr/x/gh-checks"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/tui/art"
	"github.com/dlvhdr/gh-enhance/internal/tui/scrollbar"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type errMsg error

type pane int

const (
	PaneRuns pane = iota
	PaneJobs
	PaneSteps
	PaneChecks
	PaneLogs
)

type model struct {
	width             int
	height            int
	prNumber          string
	repo              string
	pr                api.PR
	prWithChecks      api.PRWithChecks
	workflowRuns      []data.WorkflowRun
	runsList          list.Model
	jobsList          list.Model
	stepsList         list.Model
	checksList        list.Model
	logsViewport      viewport.Model
	numHighlights     int
	scrollbar         tea.Model
	focusedPane       pane
	zoomedPane        *pane
	err               error
	runsDelegate      list.ItemDelegate
	jobsDelegate      list.ItemDelegate
	stepsDelegate     list.ItemDelegate
	checksDelegate    list.ItemDelegate
	styles            styles
	logsSpinner       spinner.Model
	logsInput         textinput.Model
	inProgressSpinner spinner.Model
	flat              bool
	lastTick          time.Time
	version           string
	rateLimit         api.RateLimit
	lastFetched       time.Time
	help              help.Model
}

type ModelOpts struct {
	Flat bool
}

func NewModel(repo string, number string, opts ModelOpts) model {
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
	runsList.SetWidth(focusedLargePaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList(s)
	jobsList.Title = makePill(ListSymbol+" Jobs", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(unfocusedLargePaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList(s)
	stepsList.Title = makePill(ListSymbol+" Steps", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(unfocusedLargePaneWidth)

	checksList, checksDelegate := newChecksDefaultList(s)
	checksList.Title = makePill(ListSymbol+" checks", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	checksList.SetStatusBarItemName("step", "checks")
	checksList.SetWidth(unfocusedLargePaneWidth)

	vp := viewport.New()
	vp.LeftGutterFunc = func(info viewport.GutterContext) string {
		return lipgloss.NewStyle().Foreground(s.colors.faintColor).Render(
			fmt.Sprintf(" %*d %s ", 5, info.Index+1,
				lipgloss.NewStyle().Foreground(s.colors.fainterColor).Render("│")))
	}
	vp.KeyMap.Right = rightKey
	vp.KeyMap.Left = leftKey

	vp.HighlightStyle = lipgloss.NewStyle().Foreground(s.tint.Black).Background(s.tint.Blue)
	vp.SelectedHighlightStyle = lipgloss.NewStyle().Foreground(s.tint.Black).Background(s.tint.BrightGreen)

	sb := scrollbar.NewVertical()
	sb.Style = sb.Style.Inherit(s.scrollbarStyle)
	sb.ThumbStyle = sb.ThumbStyle.Inherit(s.scrollbarThumbStyle)
	sb.TrackStyle = sb.TrackStyle.Inherit(s.scrollbarTrackStyle)

	ls := spinner.New(spinner.WithSpinner(LogsFrames))
	ls.Style = s.faintFgStyle

	li := textinput.New()
	li.SetWidth(20)
	li.Styles.Cursor = textinput.CursorStyle{
		Color: s.colors.faintColor,
		Shape: tea.CursorBar,
		Blink: false,
	}
	li.VirtualCursor = true
	li.Prompt = " "
	li.Placeholder = "Search..."
	li.Styles.Focused = textinput.StyleState{
		Text:        lipgloss.NewStyle(),
		Placeholder: s.faintFgStyle,
		Prompt:      s.faintFgStyle,
	}

	li.Styles.Blurred = textinput.StyleState{
		Text:        lipgloss.NewStyle(),
		Placeholder: s.faintFgStyle,
		Prompt:      s.faintFgStyle,
	}

	ips := spinner.New(spinner.WithSpinner(InProgressFrames))
	ips.Style = lipgloss.NewStyle().Foreground(s.colors.warnColor)

	h := help.New()
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(s.colors.lightColor)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(s.tint.BrightWhite)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Blue)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(lipgloss.Blue)

	focusedPane := PaneRuns
	if opts.Flat {
		focusedPane = PaneChecks
	}

	m := model{
		jobsList:          jobsList,
		runsList:          runsList,
		stepsList:         stepsList,
		checksList:        checksList,
		prNumber:          number,
		repo:              repo,
		runsDelegate:      runsDelegate,
		jobsDelegate:      jobsDelegate,
		stepsDelegate:     stepsDelegate,
		checksDelegate:    checksDelegate,
		logsViewport:      vp,
		scrollbar:         sb,
		styles:            s,
		logsSpinner:       ls,
		logsInput:         li,
		help:              h,
		version:           version,
		inProgressSpinner: ips,
		flat:              opts.Flat,
		focusedPane:       focusedPane,
		lastFetched:       time.Now(),
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	return m.makeInitCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	if _, ok := msg.(spinner.TickMsg); !ok {
		log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	}
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)

	// startIntervalFetching is sent after the refreshInterval duration has elapsed
	// at this point m.fetchPRChecksWithInterval() checks if all checks have concluded
	// if they did - it's a noop, otherwise we check at the interval
	// m.fetchPRChecksWithInterval needs an up to date model, so this *cannot* be called
	// at the `m.makeInitCmd`. The up to date model is received by this `Update` func.
	case startIntervalFetching:
		cmds = append(cmds, m.fetchPRChecksWithInterval())

	case prFetchedMsg:
		m.pr = msg.pr

	case workflowRunsFetchedMsg, prChecksIntervalTickMsg:
		var wrMsg workflowRunsFetchedMsg
		if tickMsg, ok := msg.(prChecksIntervalTickMsg); ok {
			wrMsg = tickMsg.msg.(workflowRunsFetchedMsg)
		} else {
			wrMsg = msg.(workflowRunsFetchedMsg)
		}
		m.rateLimit = wrMsg.rateLimit
		if wrMsg.err != nil && wrMsg.rateLimit.Remaining == 0 {
			log.Warn("rate limit reached, waiting", "rateLimit", wrMsg.rateLimit)
			return m, nil
		}

		log.Debug("workflow runs fetched", "rateLimit", wrMsg.rateLimit)

		m.prWithChecks = wrMsg.pr
		if _, ok := msg.(prChecksIntervalTickMsg); ok {
			m.lastFetched = time.Now()
			cmds = append(cmds, m.fetchPRChecksWithInterval())
		}

		if len(wrMsg.pr.Commits.Nodes) > 0 {
			pageInfo := wrMsg.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.PageInfo
			if !pageInfo.HasPreviousPage {
				m.workflowRuns = make([]data.WorkflowRun, 0)
			}

			m.mergeWorkflowRuns(wrMsg)

			if pageInfo.HasNextPage {
				log.Info("fetching next checks page", "pageInfo", pageInfo)
				cmds = append(cmds, m.makeGetNextPagePRChecksCmd(pageInfo.EndCursor))
			} else {
				m.stopSpinners()
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
			return m, tea.Sequence(tea.ExitAltScreen, msgCmd, tea.Quit)
		}

	case workflowRunStepsFetchedMsg:
		cmds = append(cmds, m.enrichRunWithJobsStepsV2(msg)...)
		cmds = append(cmds, m.updateLists()...)

	case checkStepsFetchedMsg:
		m.enrichCheckWithSteps(msg)
		cmds = append(cmds, m.updateLists()...)

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

			cmds = append(cmds, m.updateLists()...)
		}

	case checkRunOutputFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			if ji.job.Id == msg.jobId {
				ji.renderedText = msg.renderedText
				ji.loadingLogs = false
				currJob := m.jobsList.SelectedItem()
				if currJob != nil && currJob.(*jobItem).job.Id == msg.jobId {
					cmds = append(cmds, m.renderJobLogs())
				}

				cmds = append(cmds, m.updateLists()...)
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
		cmds = append(cmds, m.fetchPRChecksWithInterval())

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

		log.Info("👤 key pressed", "key", msg.String())
		if m.checksList.FilterState() == list.Filtering ||
			m.runsList.FilterState() == list.Filtering ||
			m.jobsList.FilterState() == list.Filtering ||
			m.stepsList.FilterState() == list.Filtering {
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
			m.flat = !m.flat
			if m.flat {
				m.focusedPane = PaneChecks
			} else {
				m.focusedPane = PaneRuns
			}
			cmds = append(cmds, m.onWorkflowRunsFetched()...)
			if m.flat {
				cmds = append(cmds, m.onCheckChanged()...)
			} else {
				cmds = append(cmds, m.onRunChanged()...)
			}
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
			newModel := NewModel(m.repo, m.prNumber, ModelOpts{})
			newModel.width = m.width
			newModel.height = m.height
			newModel.setHeights()
			newModel.setWidths()

			newModel.setFocusedPaneStyles()

			m.lastFetched = time.Now()
			return newModel, newModel.makeInitCmd()
		}

		if key.Matches(msg, rerunKey) {
			if m.focusedPane != PaneRuns && m.focusedPane != PaneJobs && m.focusedPane != PaneChecks {
				break
			}

			ri := m.getSelectedRunItem()
			if m.focusedPane == PaneRuns && ri != nil {
				cmds = append(cmds, m.rerunRun(ri.run.Id)...)
			} else {
				ji := m.getSelectedJobItem()
				if ri == nil || ji == nil {
					break
				}
				cmds = append(cmds, m.rerunJob(ri.run.Id, ji.job.Id)...)
			}
		}

		if key.Matches(msg, helpKey) {
			m.help.ShowAll = !m.help.ShowAll
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
		checks := m.checksList.Items()
		for _, run := range checks {
			ci := run.(*checkItem)
			if ci != nil && ci.isStatusInProgress() {
				ci.spinner, cmd = ci.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		runs := m.runsList.Items()
		for _, run := range runs {
			ri := run.(*runItem)
			if ri != nil && ri.IsInProgress() {
				ri.spinner, cmd = ri.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		jobs := m.jobsList.Items()
		for _, job := range jobs {
			ji := job.(*jobItem)
			if ji != nil && ji.isStatusInProgress() {
				ji.spinner, cmd = ji.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

		steps := m.stepsList.Items()
		for _, step := range steps {
			si := step.(*stepItem)
			if si != nil && si.IsInProgress() {
				si.spinner, cmd = si.spinner.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

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

	case errMsg:
		m.err = msg
		return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)
	}

	switch m.focusedPane {
	case PaneChecks:
		before := m.checksList.GlobalIndex()
		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.checksList.GlobalIndex()
		if before != after {
			cmds = append(cmds, m.onCheckChanged()...)
			cmds = append(cmds, m.updateLists()...)
		}
	case PaneRuns:
		before := m.runsList.GlobalIndex()
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.runsList.GlobalIndex()
		if before != after {
			cmds = append(cmds, m.onRunChanged()...)
			cmds = append(cmds, m.updateLists()...)
		}
	case PaneJobs:
		before := m.jobsList.GlobalIndex()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.GlobalIndex()
		if before != after {
			cmds = append(cmds, m.onJobChanged()...)
		}
	case PaneSteps:
		before := m.stepsList.GlobalIndex()
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.stepsList.GlobalIndex()
		if before != after {
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

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		log.Error("fatal error", "err", m.err)
		return m.err.Error()
	}

	header := m.viewHeader()
	footer := m.viewFooter()

	panes := ""
	if m.flat {
		panes = m.viewFlatChecks()
	} else {
		panes = m.viewHierarchicalChecks()
	}

	if m.help.ShowAll {
		help := m.styles.helpPaneStyle.Width(m.width).Render(m.help.View(keys))
		footer = lipgloss.JoinVertical(lipgloss.Left, help, footer)
	}

	rootStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height)
	return rootStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		panes,
		footer,
	))
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
	} else if m.width != 0 && m.width <= smallScreen {
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
	} else if m.width != 0 && m.width <= smallScreen {
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

	status := bgStyle.Render(m.viewCommitStatus(bgStyle))
	prWidth := m.width - lipgloss.Width(status) - logoWidth -
		m.styles.headerStyle.GetHorizontalFrameSize()
	title := ""
	if m.pr.Title != "" {
		title = bgStyle.Width(prWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
			m.viewRepo(prWidth, bgStyle),
			m.viewPRName(prWidth, bgStyle),
		))
	} else {
		title = bgStyle.Width(prWidth).Render(fmt.Sprintf("Loading %s PR #%s...", m.repo, m.prNumber))
	}

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, status, bgStyle.Render(title), logo))
}

func (m *model) viewRepo(width int, bgStyle lipgloss.Style) string {
	status := ""
	if m.pr.Merged {
		status = makePill(fmt.Sprintf("%s %s", MergedIcon, "Merged"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.mergedColor)
	} else if m.pr.IsDraft {
		status = makePill(fmt.Sprintf("%s %s", DraftIcon, "Draft"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.whiteColor)
	} else if m.pr.Closed {
		status = makePill(fmt.Sprintf("%s %s", ClosedIcon, "Closed"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.errorColor)
	} else {
		status = makePill(fmt.Sprintf("%s %s", OpenIcon, "Open"),
			lipgloss.NewStyle().Foreground(m.styles.colors.darkerColor), m.styles.colors.successColor)
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

func (m *model) viewFooter() string {
	bg := lipgloss.NewStyle().Background(m.styles.footerStyle.GetBackground())
	sFooter := m.styles.footerStyle.Width(m.width)

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

	stats := checks.AccumulatedStats(contexts.CheckRunCountsByState, contexts.StatusContextCountsByState)

	if stats.Failed > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.errorColor).Render(
			fmt.Sprintf("%d failing", stats.Failed)))
	}
	if stats.InProgress > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.warnColor).Render(
			fmt.Sprintf("%d in progress", stats.InProgress)))
	}
	if stats.Succeeded > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.successColor).Render(
			fmt.Sprintf("%d successful", stats.Succeeded)))
	}
	if stats.Skipped > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.faintColor).Render(
			fmt.Sprintf("%d skipped", stats.Skipped)))
	}

	checks := bg.Render(strings.Join(texts, bg.Render(", ")))

	reFetchingIn := ""
	if m.prWithChecks.Number != 0 && m.prWithChecks.IsStatusCheckInProgress() {
		until := time.Until(m.lastFetched.Add(refreshInterval)).Truncate(time.Second).Seconds()
		untilStr := fmt.Sprintf("in %ds", int(until))
		if until <= 0 {
			untilStr = "now..."
		}
		reFetchingIn = bg.Padding(0, 1).Foreground(m.styles.colors.faintColor).Render(fmt.Sprintf("Refreshing %s", untilStr))
	}

	help := m.styles.helpButtonStyle.Render("? help")

	gap := bg.Render(
		strings.Repeat(" ", m.width-lipgloss.Width(totalText)-lipgloss.Width(checks)-
			lipgloss.Width(reFetchingIn)-lipgloss.Width(help)-
			m.styles.footerStyle.GetHorizontalFrameSize()))

	return sFooter.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, totalText, checks, gap, reFetchingIn, help))
}

func (m *model) shouldShowSteps() bool {
	loadingSteps := false
	if m.flat {
		check := m.checksList.SelectedItem()
		if check != nil {
			ci := check.(*checkItem)
			loadingSteps = ci.loadingSteps
		}
	} else {
		job := m.jobsList.SelectedItem()
		if job != nil {
			ji := job.(*jobItem)
			loadingSteps = ji.loadingSteps
		}
	}

	return loadingSteps || len(m.stepsList.VisibleItems()) > 0
}

func (m *model) viewLogs() string {
	title := "Job Logs"
	w := m.logsWidth()
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
		inputView = lipgloss.NewStyle().Width(w).Border(lipgloss.RoundedBorder(), true).BorderForeground(
			m.styles.colors.fainterColor).Render(m.logsInput.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, inputView, m.logsContentView())
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
		m.checksDelegate.(*checksDelegate).focused = true
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = true
		m.setListUnfocusedStyles(&m.checksList, &m.checksDelegate)
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListFocusedStyles(&m.stepsList, &m.stepsDelegate, PaneSteps)
	case PaneLogs:
		m.checksDelegate.(*checksDelegate).focused = true
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
	if m.width != 0 && m.width <= smallScreen {
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
	if m.width > smallScreen {
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
	l.Styles.StatusBarActiveFilter = l.Styles.StatusBarActiveFilter.Foreground(styles.colors.faintColor)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.Foreground(styles.colors.faintColor)
	l.Styles.NoItems = l.Styles.NoItems.Width(unfocusedLargePaneWidth).Foreground(styles.colors.faintColor)
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.colors.faintColor).MarginLeft(1).MarginBottom(1)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1)
	l.SetSpinner(spinner.Dot)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.StartSpinner()
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return l
}

func (m *model) updateLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if m.flat {
		cCmds := m.updateChecksList()
		cmds = append(cmds, cCmds...)
	} else {
		rCmds := m.updateRunsList()
		cmds = append(cmds, rCmds...)

		jCmds := m.updateJobsList()
		cmds = append(cmds, jCmds...)
	}

	// the steps list is used in both modes
	sCmds := m.updateStepsList()
	cmds = append(cmds, sCmds...)

	return cmds
}

func (m *model) updateChecksList() []tea.Cmd {
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
	if len(m.checksList.VisibleItems()) > 0 || m.checksList.FilterState() == list.FilterApplied {
		m.checksList.SetShowStatusBar(true)
	} else {
		m.checksList.SetShowStatusBar(false)
	}

	return cmds
}

func (m *model) updateRunsList() []tea.Cmd {
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

	if ri.loading {
		cmds = append(cmds, m.stepsList.StartSpinner())
	} else {
		m.stepsList.StopSpinner()
	}
	if len(m.runsList.VisibleItems()) > 0 || m.runsList.FilterState() == list.FilterApplied {
		m.runsList.SetShowStatusBar(true)
	} else {
		m.runsList.SetShowStatusBar(false)
	}

	return cmds
}

func (m *model) updateJobsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getSelectedRunItem()
	if ri == nil {
		return cmds
	}

	jobs := make([]list.Item, 0)
	for _, ji := range ri.jobsItems {
		jobs = append(jobs, ji)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobs))
	if len(m.jobsList.VisibleItems()) > 0 || m.jobsList.FilterState() == list.FilterApplied {
		m.jobsList.SetShowStatusBar(true)
	} else {
		m.jobsList.SetShowStatusBar(false)
	}

	return cmds
}

func (m *model) updateStepsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	var ji *jobItem
	if ci := m.getSelectedCheckItem(); ci != nil && m.flat {
		ji = &ci.jobItem
	} else {
		ji = m.getSelectedJobItem()
	}

	existing := m.stepsList.Items()
	existingCount := len(existing)
	if ji != nil {
		for i, si := range ji.steps {
			if i < existingCount {
				cmds = append(cmds, m.stepsList.SetItem(i, si))
			} else {
				cmds = append(cmds, m.stepsList.InsertItem(i, si))
			}
		}

		for i := existingCount; i >= len(ji.steps); i-- {
			m.stepsList.RemoveItem(i)
		}
	}

	if len(m.stepsList.VisibleItems()) > 0 || m.stepsList.FilterState() == list.FilterApplied {
		m.stepsList.SetShowStatusBar(true)
	} else {
		m.stepsList.SetShowStatusBar(false)
	}

	return cmds
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
			return nil
		}
		ji, ok := job.(*jobItem)
		if !ok {
			return nil
		}
		return ji
	}
}

func (m *model) logsWidth() int {
	if m.width == 0 {
		return 0
	}

	if m.zoomedPane != nil && *m.zoomedPane == PaneLogs {
		return m.width - 1
	}

	var borders int
	if m.width != 0 && m.width <= smallScreen {
		borders = 1
	} else if m.flat {
		borders = 1
	} else {
		borders = 2
	}

	scbar := 0
	ji := m.getSelectedJobItem()
	if ji != nil && (len(ji.renderedLogs) > 0 || len(ji.renderedText) > 0) && m.isScrollbarVisible() {
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
	emptySetArt := ""
	for _, char := range art.EmptySet {
		if char == '╱' {
			emptySetArt += lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Render("╱")
		} else {
			emptySetArt += m.styles.watermarkIllustrationStyle.Render(string(char))
		}
	}

	return m.fullScreenMessageView(
		lipgloss.JoinVertical(
			lipgloss.Center,
			emptySetArt,
			m.styles.noLogsStyle.Render(message),
		),
	)
}

func (m *model) isScrollbarVisible() bool {
	return m.logsViewport.TotalLineCount() > m.logsViewport.VisibleLineCount()
}

func (m *model) enrichRunWithJobsStepsV2(msg workflowRunStepsFetchedMsg) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	jobsMap := make(map[string]api.CheckRunWithSteps)
	checks := msg.data.Resource.WorkflowRun.CheckSuite.CheckRuns.Nodes
	for _, check := range checks {
		jobsMap[fmt.Sprintf("%d", check.DatabaseId)] = check
	}

	ri := m.getRunItemById(msg.runId)
	if ri == nil {
		log.Error("run not found when trying to enrich with steps", "msg.runId", msg.runId)
		return cmds
	}

	selectedJob := m.getSelectedJobItem()
	ri.loading = false
	for jIdx, ji := range ri.jobsItems {
		ri.jobsItems[jIdx].loadingSteps = false
		jobWithSteps, ok := jobsMap[ji.job.Id]
		if !ok {
			continue
		}

		steps := make([]*stepItem, 0)
		for _, step := range jobWithSteps.Steps.Nodes {
			si := NewStepItem(step, jobWithSteps.Url, m.styles)
			if selectedJob != nil && selectedJob.job.Id == ji.job.Id {
				cmds = append(cmds, si.Tick())
			}

			steps = append(steps, &si)
		}

		ri.jobsItems[jIdx].steps = steps
	}

	return cmds
}

func (m *model) enrichCheckWithSteps(msg checkStepsFetchedMsg) {
	ci := m.getCheckItemById(msg.checkId)
	if ci == nil {
		log.Error("check not found when trying to enrich with steps", "msg", msg)
		return
	}

	ci.loadingSteps = false

	steps := make([]*stepItem, 0)
	for _, step := range msg.steps {
		si := NewStepItem(step, ci.job.Link, m.styles)
		steps = append(steps, &si)
	}

	ci.steps = steps
}

func (m *model) onCheckChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.updateStepsList()...)
	cmds = append(cmds, m.tickSteps()...)
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currCheck := m.getSelectedCheckItem()
	if currCheck == nil {
		log.Error("check changed but current check is nil")
		return nil
	}

	if currCheck.loadingSteps {
		cmds = append(cmds, m.makeFetchCheckStepsCmd(currCheck.job.Id))
	}

	if !currCheck.initiatedLogsFetch && !currCheck.isStatusInProgress() {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	}
	cmds = append(cmds, m.onJobChanged()...)

	return cmds
}

func (m *model) onRunChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.jobsList.ResetSelected()
	m.jobsList.ResetFilter()
	newRun := m.runsList.SelectedItem()

	ri, ok := newRun.(*runItem)
	if !ok {
		log.Error("run changed but there is no run", "newRun", newRun)
		return cmds
	}

	if ri.loading {
		cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
	}

	cmds = append(cmds, m.updateLists()...)
	cmds = append(cmds, m.onJobChanged()...)

	jobs := m.jobsList.Items()
	for _, job := range jobs {
		ji := job.(*jobItem)
		cmds = append(cmds, ji.Tick())
	}

	m.logsViewport.GotoTop()

	return cmds
}

func (m *model) onJobChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.resetStepsState()
	cmds = append(cmds, m.updateStepsList()...)
	cmds = append(cmds, m.tickSteps()...)
	cmds = append(cmds, m.logsSpinner.Tick, m.inProgressSpinner.Tick)

	currJob := m.getSelectedJobItem()
	if currJob != nil && !currJob.initiatedLogsFetch && !currJob.isStatusInProgress() {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	} else if currJob == nil {
		log.Error("job changed but current job is nil")
	}

	cmds = append(cmds, m.renderJobLogs())
	m.goToErrorInLogs()

	return cmds
}

func (m *model) onStepChanged() {
	ji := m.getSelectedJobItem()
	step := m.stepsList.SelectedItem()
	cursor := m.stepsList.Cursor()

	if step == nil {
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
					fmt.Sprintf("No checks reported on the '%s' branch", m.prWithChecks.HeadRefName))))
	}

	ji := m.getSelectedJobItem()
	if ji == nil {
		return m.fullScreenMessageView(m.styles.faintFgStyle.Bold(true).Render("Nothing selected..."))
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

		return m.fullScreenMessageView(m.renderFullScreenLogsSpinner(text, "view the job on github.com"))
	}

	if ji.loadingLogs || ji.loadingSteps {
		return m.loadingLogsView()
	}

	if ji.job.Bucket == data.CheckBucketCancel {
		return m.fullScreenMessageView(lipgloss.JoinVertical(lipgloss.Center,
			m.styles.faintFgStyle.Render(art.StopSign),
			m.styles.faintFgStyle.Bold(true).Render("This job was cancelled")))
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "HTTP 410:") {
		return m.fullScreenMessageView("The logs for this run have expired and are no longer available.")
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

func (m *model) getRunItemByName(runName string) *runItem {
	for _, run := range m.runsList.Items() {
		ri := run.(*runItem)
		if ri.run.Name == runName {
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
	if m.width > smallScreen {
		if len(l.Items()) == 0 {
			return unfocusedLargePaneWidth
		}
		return focusedLargePaneWidth
	}

	return focusedSmallPaneWidth
}

func (m *model) getPaneTitle(l *list.Model) string {
	if m.width != 0 && m.width <= smallScreen {
		s := m.styles.focusedPaneTitleStyle.Bold(false).UnsetBackground()
		switch m.focusedPane {
		case PaneRuns:
			return lipgloss.JoinHorizontal(lipgloss.Top,
				makePill(s.Bold(true).Render("Runs"), l.Styles.Title,
					m.styles.colors.focusedColor), s.Render(" > Jobs > Steps"))
		case PaneJobs:
			return lipgloss.JoinHorizontal(lipgloss.Top, s.Render("Runs > "),
				makePill(s.Bold(true).Render("Jobs"), l.Styles.Title,
					m.styles.colors.focusedColor), s.Render(" > Steps"))
		case PaneSteps:
			return lipgloss.JoinHorizontal(lipgloss.Top, s.Render("Runs > Jobs > "),
				makePill(s.Bold(true).Render("Steps"), l.Styles.Title, m.styles.colors.focusedColor))
		case PaneLogs:
			return ""
		}
	}

	_, itemsName := l.StatusBarItemName()
	return strings.ToUpper(string(itemsName[0])) + itemsName[1:]
}

func (m *model) getUnfocusedPaneWidth() int {
	if m.width != 0 && m.width <= smallScreen {
		return 0
	}

	return unfocusedLargePaneWidth
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
	h := m.height - headerHeight - footerHeight
	if m.help.ShowAll {
		h -= lipgloss.Height(m.help.View(keys)) + m.styles.helpPaneStyle.GetVerticalFrameSize()
	}
	return h
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
	m.help.Width = m.width
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
		selectedCheck := m.checksList.SelectedItem()
		var before *checkItem
		if selectedCheck != nil {
			before = selectedCheck.(*checkItem)
		}

		cmds = append(cmds, m.buildFlatChecksLists()...)

		if len(m.checksList.Items()) > 0 {
			cmds = append(cmds, m.onCheckChanged()...)
		}

		if before != nil && !before.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}
	} else {
		selectedRun := m.runsList.SelectedItem()
		var before *runItem
		if selectedRun != nil {
			before = selectedRun.(*runItem)
		}

		cmds = append(cmds, m.buildHierachicalChecksLists()...)

		if len(m.runsList.Items()) > 0 {
			ri := m.runsList.SelectedItem().(*runItem)
			cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
			if before == nil || before.run.Id != ri.run.Id {
				cmds = append(cmds, m.onRunChanged()...)
			}
		}

		currJob := m.getSelectedJobItem()
		if currJob != nil && !currJob.initiatedLogsFetch {
			cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
		}
	}

	cmds = append(cmds, m.updateLists()...)

	return cmds
}

func (m *model) buildFlatChecksLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	sorted := make([]data.WorkflowJob, 0)
	for _, run := range m.workflowRuns {
		sorted = append(sorted, run.Jobs...)
	}
	data.SortJobs(sorted)

	items := make([]list.Item, 0)
	for _, job := range sorted {
		ci := NewCheckItem(job, m.styles)
		items = append(items, &ci)
		cmds = append(cmds, ci.Tick())
	}
	m.checksList.SetItems(items)
	return cmds
}

func (m *model) buildHierachicalChecksLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	for i, run := range m.workflowRuns {
		ri := m.getRunItemByName(run.Name)
		if ri == nil {
			nr := NewRunItem(run, m.styles)
			ri = &nr
			cmds = append(cmds, ri.Tick())
			cmds = append(cmds, m.runsList.InsertItem(i, ri))
		}
		ri.run = &run

		jobs := make([]*jobItem, 0)
		for _, job := range run.Jobs {
			ji := m.getJobItemById(job.Id)
			if ji == nil {
				nji := NewJobItem(job, m.styles)
				cmds = append(cmds, nji.Tick(), m.inProgressSpinner.Tick)
				ji = &nji
			}
			ji.job = &job
			jobs = append(jobs, ji)
		}

		ri.jobsItems = jobs
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
		return bgStyle.Padding(0, 1).BorderForeground(m.styles.colors.darkColor).BorderBackground(bgStyle.GetBackground()).Border(
			lipgloss.ThickBorder(), false, true, false, false).Render(res)
	}

	return string(status)
}

func (m *model) paneStyle(pane pane) lipgloss.Style {
	// the border of the pane is the actually rendered by the previous pane
	if m.previousPane() == pane {
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

func (m *model) tickSteps() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	steps := m.stepsList.Items()
	for _, step := range steps {
		si := step.(*stepItem)
		cmds = append(cmds, si.Tick())
	}
	return cmds
}
