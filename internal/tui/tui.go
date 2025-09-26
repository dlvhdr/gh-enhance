package tui

import (
	"fmt"
	"math"
	"os"
	"regexp"
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
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/tui/art"
	"github.com/dlvhdr/gh-enhance/internal/tui/scrollbar"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type errMsg error

type focusedPane int

const (
	PaneRuns focusedPane = iota
	PaneJobs
	PaneSteps
	PaneLogs
)

type model struct {
	width             int
	height            int
	prNumber          string
	repo              string
	pr                api.PR
	runsList          list.Model
	jobsList          list.Model
	stepsList         list.Model
	logsViewport      viewport.Model
	numHighlights     int
	scrollbar         tea.Model
	focusedPane       focusedPane
	err               error
	runsDelegate      list.ItemDelegate
	jobsDelegate      list.ItemDelegate
	stepsDelegate     list.ItemDelegate
	styles            styles
	logsSpinner       spinner.Model
	logsInput         textinput.Model
	inProgressSpinner spinner.Model
	lastTick          time.Time
	help              help.Model
}

func NewModel(repo string, number string) model {
	tint.NewDefaultRegistry()
	tint.SetTint(tint.TintTokyoNightStorm)
	theme := os.Getenv("ENHANCE_THEME")
	if theme != "" {
		tint.SetTintID(theme)
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

	m := model{
		jobsList:          jobsList,
		runsList:          runsList,
		stepsList:         stepsList,
		prNumber:          number,
		repo:              repo,
		runsDelegate:      runsDelegate,
		jobsDelegate:      jobsDelegate,
		stepsDelegate:     stepsDelegate,
		logsViewport:      vp,
		scrollbar:         sb,
		styles:            s,
		logsSpinner:       ls,
		logsInput:         li,
		help:              h,
		inProgressSpinner: ips,
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

	case workflowRunsFetchedMsg:
		m.pr = msg.pr
		m.runsList.StopSpinner()
		m.jobsList.StopSpinner()

		if msg.err != nil {
			log.Debug("error when fetching workflow runs", "err", msg.err)
			m.err = msg.err
			msgCmd := tea.Printf("%s\nrepo=%s, number=%s\n",
				lipgloss.NewStyle().Foreground(m.styles.colors.errorColor).Bold(true).Render(
					"❌ Pull request not found."), m.repo, m.prNumber)
			return m, tea.Sequence(tea.ExitAltScreen, msgCmd, tea.Quit)
		}

		cmds = append(cmds, m.onWorkflowRunsFetched(msg)...)

	case workflowRunStepsFetchedMsg:
		cmds = append(cmds, m.enrichRunWithJobsStepsV2(msg)...)
		cmds = append(cmds, m.updateLists()...)

	case jobLogsFetchedMsg:
		ji := m.getJobItemById(msg.jobId)
		if ji != nil {
			ji.logs = msg.logs
			ji.logsErr = msg.err
			ji.logsStderr = msg.stderr
			ji.loadingLogs = false
			currJob := m.jobsList.SelectedItem()
			if currJob != nil && currJob.(*jobItem).job.Id == msg.jobId {
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

		cmds = append(cmds, tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
			return m.fetchPRChecks(m.prNumber)
		}))

	case reRunRunMsg:
		if msg.err != nil {
			log.Error("error rerunning run", "jobId", msg.runId, "err", msg.err)
		}
		ri := m.getRunItemById(msg.runId)
		if ri == nil {
			break
		}

		cmds = append(cmds, tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
			return m.fetchPRChecks(m.prNumber)
		}))

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
		if m.runsList.FilterState() == list.Filtering ||
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

		if key.Matches(msg, refreshAllKey) {
			newModel := NewModel(m.repo, m.prNumber)
			newModel.width = m.width
			newModel.height = m.height
			newModel.setHeights()
			newModel.setWidths()

			newModel.setFocusedPaneStyles()

			return newModel, newModel.makeInitCmd()
		}

		if key.Matches(msg, rerunKey) {
			if m.focusedPane != PaneRuns && m.focusedPane != PaneJobs {
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

		if key.Matches(msg, openPRKey) && m.pr.Url != "" {
			cmds = append(cmds, makeOpenUrlCmd(m.pr.Url))
		}

		if key.Matches(msg, nextPaneKey) {
			pane := m.focusedPane + 1
			if pane == PaneSteps && !m.shouldShowSteps() {
				pane = pane + 1
			}
			m.focusedPane = min(PaneLogs, pane)
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, prevPaneKey) {
			pane := m.focusedPane - 1
			if pane == PaneSteps && !m.shouldShowSteps() {
				pane = pane - 1
			}
			m.focusedPane = max(PaneRuns, pane)
			m.setFocusedPaneStyles()
		}

	case spinner.TickMsg:
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

		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case errMsg:
		m.err = msg
		return m, nil
	}

	switch m.focusedPane {
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

	cmds = append(cmds, cmd)
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

	rootStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height)

	header := m.viewHeader()
	footer := m.viewFooter()

	runsPane := makePointingBorder(m.paneStyle(PaneRuns).Render(m.runsList.View()))
	jobsPane := makePointingBorder(m.paneStyle(PaneJobs).Render(m.jobsList.View()))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.paneStyle(PaneSteps).Render(m.stepsList.View()))
	}

	panes := make([]string, 0)
	if m.width != 0 && m.width <= smallScreen {
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
	} else {
		panes = append(panes, runsPane)
		panes = append(panes, jobsPane)
		panes = append(panes, stepsPane)
	}
	panes = append(panes, m.viewLogs())

	if m.help.ShowAll {
		help := m.styles.helpPaneStyle.Width(m.width).Render(m.help.View(keys))
		footer = lipgloss.JoinVertical(lipgloss.Left, help, footer)
	}

	return rootStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			panes...,
		),
		footer,
	))
}

func (m *model) viewHeader() string {
	s := lipgloss.NewStyle().Background(m.styles.headerStyle.GetBackground())
	version := s.Height(lipgloss.Height(Logo)).Render(" \n v0.1.0")

	logoWidth := lipgloss.Width(Logo) + lipgloss.Width(version)
	logo := lipgloss.PlaceHorizontal(
		logoWidth,
		lipgloss.Right,
		s.Width(logoWidth).Render(
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				m.styles.logoStyle.Render(Logo),
				version,
			)))

	status := s.Render(m.viewCommitStatus(s))
	prWidth := m.width - lipgloss.Width(status) - logoWidth -
		m.styles.headerStyle.GetHorizontalFrameSize()
	pr := s.Width(prWidth).Render(fmt.Sprintf("Loading %s PR #%s...", m.repo, m.prNumber))
	if m.pr.Title != "" {
		pr = s.Width(prWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
			m.viewRepo(prWidth, s),
			m.viewPRName(prWidth, s),
		))
	}

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, status, s.Render(pr), logo))
}

func (m *model) viewRepo(width int, bgStyle lipgloss.Style) string {
	return bgStyle.Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top,
		bgStyle.Render(" "),
		bgStyle.Foreground(m.styles.colors.darkColor).Bold(true).Render(m.pr.Repository.NameWithOwner),
		bgStyle.Render(" "),
		bgStyle.Foreground(m.styles.colors.faintColor).Render(fmt.Sprintf("#%d", m.pr.Number)),
	))
}

func (m *model) viewPRName(width int, bgStyle lipgloss.Style) string {
	mergeStatus := ""
	if m.pr.Merged {
		mergeStatus = m.styles.mergedGlyph.Render()
	} else if m.pr.IsDraft {
		mergeStatus = m.styles.draftGlyph.Render()
	} else if m.pr.Closed {
		mergeStatus = m.styles.closedGlyph.Render()
	} else {
		mergeStatus = m.styles.openGlyph.Render()
	}

	return bgStyle.Width(width).Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		bgStyle.Render(mergeStatus),
		bgStyle.Render(" "),
		bgStyle.Bold(true).Render(m.pr.Title)),
	)
}

func (m *model) viewFooter() string {
	if m.width == 0 {
		return ""
	}

	failingChecks, successfulChecks, skippedChecks, inProgressChecks := 0, 0, 0, 0
	failingContext := 0
	if len(m.pr.Commits.Nodes) == 0 {
		return ""
	}
	for _, item := range m.runsList.Items() {
		ri := item.(*runItem)
		for _, ji := range ri.jobsItems {
			switch ji.job.Bucket {
			case data.CheckBucketPass:
				successfulChecks += 1
			case data.CheckBucketFail:
				failingChecks += 1
			case data.CheckBucketSkipping:
				skippedChecks += 1
			case data.CheckBucketCancel:
				skippedChecks += 1
			default:
				inProgressChecks += 1
			}
		}
	}

	// for _, count := range m.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.StatusContextCountsByState {
	// 	switch count.State {
	// 	case api.ConclusionFailure:
	// 		failingContext += count.Count
	// 	case api.ConclusionActionRequired:
	// 	case api.ConclusionCancelled:
	// 	case api.ConclusionNeutral:
	// 	case api.ConclusionStartupFailure:
	// 	case api.ConclusionTimedOut:
	// 		failingContext += count.Count
	// 	}
	// }

	texts := make([]string, 0)
	bg := lipgloss.NewStyle().Background(m.styles.footerStyle.GetBackground())
	if failingChecks > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.errorColor).Render(
			fmt.Sprintf("%d failing", failingChecks)))
	}
	if inProgressChecks > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.warnColor).Render(
			fmt.Sprintf("%d in progress", inProgressChecks)))
	}
	if successfulChecks > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.successColor).Render(
			fmt.Sprintf("%d successful", successfulChecks)))
	}
	if skippedChecks > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.faintColor).Render(
			fmt.Sprintf("%d skipped", skippedChecks)))
	}

	if failingContext > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.errorColor).Render(
			fmt.Sprintf("%d failing contexts", failingContext)))
	}

	checks := bg.Render(strings.Join(texts, bg.Render(", ")))

	help := m.styles.helpButtonStyle.Render("? help")

	return m.styles.footerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, checks, bg.Render(
			strings.Repeat(" ", m.width-lipgloss.Width(checks)-lipgloss.Width(help)-
				m.styles.footerStyle.GetHorizontalFrameSize())), help))
}

func (m *model) shouldShowSteps() bool {
	job := m.jobsList.SelectedItem()
	loadingSteps := false
	if job != nil {
		ji := job.(*jobItem)
		loadingSteps = ji.loadingSteps
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
	case PaneRuns:
		m.runsDelegate.(*runsDelegate).focused = true
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListFocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneJobs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = true
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListFocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneSteps:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = true
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListFocusedStyles(&m.stepsList, &m.stepsDelegate)
	case PaneLogs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	}

	w := m.logsWidth()
	m.logsViewport.SetWidth(w)
	m.logsInput.SetWidth(int(math.Max(float64(0), float64(
		w-lipgloss.Width(m.logsInput.Prompt)-2))))
}

func (m *model) setListFocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	if m.width != 0 && m.width <= smallScreen {
		l.Styles.Title = m.styles.focusedPaneTitleStyle.Bold(false)
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle.Bold(false)
		l.Title = m.getPaneTitle(l)
	} else {
		l.Styles.Title = m.styles.focusedPaneTitleStyle
		l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
		l.Title = makePill(m.getPaneTitle(l), l.Styles.Title, m.styles.colors.focusedColor)
	}

	w := m.getFocusedPaneWidth(l)
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

	rCmds := m.updateRunsList()
	cmds = append(cmds, rCmds...)

	jCmds := m.updateJobsList()
	cmds = append(cmds, jCmds...)

	sCmds := m.updateStepsList()
	cmds = append(cmds, sCmds...)

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

	ji := m.getSelectedJobItem()
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

func (m *model) logsWidth() int {
	if m.width == 0 {
		return 0
	}

	var borders int
	if m.width != 0 && m.width <= smallScreen {
		borders = 1
	} else {
		borders = 2
	}
	sb := 0
	ji := m.getSelectedJobItem()
	if ji != nil && (len(ji.renderedLogs) > 0 || len(ji.renderedText) > 0) && m.isScrollbarVisible() {
		sb = lipgloss.Width(m.scrollbar.(scrollbar.Vertical).View())
	}
	steps := 0
	if m.shouldShowSteps() {
		steps = m.stepsList.Width()
		borders = borders + 1
	}
	return m.width - m.runsList.Width() - m.jobsList.Width() - steps - borders - sb
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
		log.Error("run not found when trying to enrich with steps", "msg", msg)
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

	m.logsViewport.ClearHighlights()
	m.numHighlights = 0
	m.logsInput.Reset()
	m.stepsList.ResetSelected()
	m.stepsList.ResetFilter()
	cmds = append(cmds, m.updateStepsList()...)

	steps := m.stepsList.Items()
	for _, step := range steps {
		si := step.(*stepItem)
		cmds = append(cmds, si.Tick())
	}

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
	job := m.jobsList.SelectedItem()
	step := m.stepsList.SelectedItem()
	cursor := m.stepsList.Cursor()

	if step == nil {
		return
	}

	if cursor == len(m.stepsList.Items())-1 {
		m.logsViewport.GotoBottom()
		return
	}

	for i, log := range job.(*jobItem).logs {
		if log.Time.After(step.(*stepItem).step.StartedAt) {
			m.logsViewport.SetYOffset(i - 1)
			return
		}
	}
}

func (m *model) renderJobLogs() tea.Cmd {
	currJob := m.jobsList.SelectedItem()
	if currJob == nil || currJob.(*jobItem).loadingLogs {
		m.logsViewport.SetContent("")
	}

	ji, ok := currJob.(*jobItem)
	if !ok {
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

	if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun || ji.job.Kind == data.JobKindExternal {
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
	if m.pr.Number != 0 && len(m.pr.Commits.Nodes) > 0 &&
		m.pr.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.CheckRunCount == 0 {
		return m.fullScreenMessageView(
			lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(m.styles.tint.BrightWhite).Render(art.CheckmarkSign),
				"",
				m.styles.faintFgStyle.Bold(true).Render(
					fmt.Sprintf("No checks reported on the '%s' branch", m.pr.HeadRefName))))
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

func (m *model) getJobItemById(jobId string) *jobItem {
	for _, run := range m.runsList.Items() {
		ri := run.(*runItem)
		for i := range ri.jobsItems {
			if ri.jobsItems[i].job.Id == jobId {
				return ri.jobsItems[i]
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

func (m *model) getFocusedPaneWidth(l *list.Model) int {
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

func (m *model) onWorkflowRunsFetched(msg workflowRunsFetchedMsg) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	selectedRun := m.runsList.SelectedItem()
	var before *runItem
	if selectedRun != nil {
		before = selectedRun.(*runItem)
	}

	for i, run := range msg.runs {
		ri := m.getRunItemById(run.Id)
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
		cmds = append(cmds, m.runsList.SetItem(i, ri))
	}

	if len(m.runsList.Items()) > 0 {
		ri := m.runsList.SelectedItem().(*runItem)
		cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
		if before == nil || before.run.Id != ri.run.Id {
			cmds = append(cmds, m.onRunChanged()...)
		}
	}

	cmds = append(cmds, m.updateLists()...)

	currJob := m.getSelectedJobItem()
	if currJob != nil && !currJob.initiatedLogsFetch {
		cmds = append(cmds, m.logsSpinner.Tick, m.makeFetchJobLogsCmd())
	}

	if m.pr.IsStatusCheckInProgress() {
		log.Debug("refetching, PR is still in progress", "state",
			m.pr.Commits.Nodes[0].Commit.StatusCheckRollup.State)
		cmds = append(cmds, tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
			return m.fetchPRChecks(m.prNumber)
		}))
	} else {
		log.Debug("stopping refetching, all tasks are done")
	}

	return cmds
}

func (m *model) viewCommitStatus(bgStyle lipgloss.Style) string {
	if len(m.pr.Commits.Nodes) == 0 {
		return ""
	}

	s := bgStyle.MaxHeight(2)
	status := m.pr.Commits.Nodes[0].Commit.StatusCheckRollup.State
	res := ""
	switch status {
	case api.CommitStateSuccess:
		res = s.Foreground(m.styles.colors.successColor).Render(art.SmallCheckmarkSign)
	case api.CommitStateError, api.CommitStateFailure:
		res = s.Foreground(m.styles.colors.errorColor).Render(art.SmallCrossSign)
	case api.CommitStateExpected, api.CommitStatePending:
		res = s.Foreground(m.styles.colors.warnColor).Render(WaitingIcon)
	}

	if res != "" {
		return bgStyle.Padding(0, 1).MaxHeight(2).Render(res)
	}

	return string(status)
}

func (m *model) paneStyle(pane focusedPane) lipgloss.Style {
	// the border of the pane is the actually rendered by the previous pane
	if m.focusedPane-1 == pane {
		return m.styles.focusedPaneStyle
	}

	return m.styles.paneStyle
}
