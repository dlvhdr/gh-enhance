package tui

import (
	"fmt"
	"math"
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

const (
	headerHeight = 4
	footerHeight = 1
	smallScreen  = 130

	unfocusedLargePaneWidth = 20
	focusedLargePaneWidth   = 40

	focusedSmallPaneWidth = 25
)

type model struct {
	width         int
	height        int
	prNumber      string
	repo          string
	pr            api.PR
	runsList      list.Model
	jobsList      list.Model
	stepsList     list.Model
	logsViewport  viewport.Model
	numHighlights int
	scrollbar     tea.Model
	focusedPane   focusedPane
	err           error
	runsDelegate  list.ItemDelegate
	jobsDelegate  list.ItemDelegate
	stepsDelegate list.ItemDelegate
	styles        styles
	logsSpinner   spinner.Model
	logsInput     textinput.Model
	help          help.Model
}

func NewModel(repo string, number string) model {
	tint.NewDefaultRegistry()
	tint.SetTint(tint.TintTokyoNightStorm)

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

	h := help.New()
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(s.colors.lightColor)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(s.tint.BrightWhite)
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Blue)
	h.Styles.Ellipsis = lipgloss.NewStyle().Foreground(lipgloss.Blue)

	m := model{
		jobsList:      jobsList,
		runsList:      runsList,
		stepsList:     stepsList,
		prNumber:      number,
		repo:          repo,
		runsDelegate:  runsDelegate,
		jobsDelegate:  jobsDelegate,
		stepsDelegate: stepsDelegate,
		logsViewport:  vp,
		scrollbar:     sb,
		styles:        s,
		logsSpinner:   ls,
		logsInput:     li,
		help:          h,
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	// return textinput.Blink
	return tea.Batch(m.runsList.StartSpinner(), m.logsSpinner.Tick, m.jobsList.StartSpinner(), m.makeGetPRChecksCmd(m.prNumber))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// m.logsViewport.SoftWrap = false
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {
	case cursor.BlinkMsg:
		m.logsInput, cmd = m.logsInput.Update(msg)
		cmds = append(cmds, cmd)

	case workflowRunsFetchedMsg:
		m.pr = msg.pr
		m.runsList.StopSpinner()
		m.jobsList.StopSpinner()
		runItems := make([]list.Item, 0)
		for _, run := range msg.runs {
			ri := NewRunItem(run, m.styles)
			runItems = append(runItems, &ri)
		}

		cmds = append(cmds, m.runsList.SetItems(runItems))
		cmds = append(cmds, m.updateLists()...)

		if len(runItems) > 0 {
			ri := runItems[0].(*runItem)
			cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
			if len(ri.run.Jobs) > 0 {
				m.jobsList.ResetSelected()
				cmds = append(cmds, m.onJobChanged()...)
			}
		}

	case workflowRunStepsFetchedMsg:
		m.enrichRunWithJobsStepsV2(msg)
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
				m.renderJobLogs()
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
					m.renderJobLogs()
				}

				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case tea.WindowSizeMsg:
		log.Debug("window size changed", "width", msg.Width, "height", msg.Height)
		m.width = msg.Width
		m.height = msg.Height
		m.setHeights()

		m.help.Width = m.width
		w := m.logsWidth()
		m.logsViewport.SetWidth(w)
		m.logsInput.SetWidth(w - 10)

		m.setFocusedPaneStyles()
	case tea.KeyPressMsg:
		if key.Matches(msg, quitKey) {
			log.Debug("quitting", "msg", msg)
			return m, tea.Quit
		}

		log.Debug("👤 key pressed", "key", msg.String())
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
		currJob := m.jobsList.SelectedItem()
		if currJob == nil || currJob.(*jobItem).loadingLogs {
			m.logsSpinner, cmd = m.logsSpinner.Update(msg)
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
		log.Debug("updating logsInput with msg", "msg", fmt.Sprintf("%+v", msg), "focused", m.logsInput.Focused())
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
		return m.err.Error()
	}

	rootStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height)

	header := m.viewHeader()
	footer := m.viewFooter()

	runsPane := makePointingBorder(m.styles.paneStyle.Render(m.runsList.View()))
	jobsPane := makePointingBorder(m.styles.paneStyle.Render(m.jobsList.View()))
	stepsPane := ""
	if m.shouldShowSteps() {
		stepsPane = makePointingBorder(m.styles.paneStyle.Render(m.stepsList.View()))
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
	version := s.Height(lipgloss.Height(Logo)).Render(" \n v0.1.0〓")

	logoWidth := lipgloss.Width(Logo) + lipgloss.Width(version)
	logo := lipgloss.PlaceHorizontal(
		logoWidth,
		lipgloss.Right,
		s.Width(logoWidth).Render(
			lipgloss.JoinHorizontal(lipgloss.Bottom,
				m.styles.logoStyle.Render(Logo),
				version,
			)))

	prWidth := m.width - logoWidth - m.styles.headerStyle.GetHorizontalFrameSize()
	pr := s.Width(prWidth).Render("Loading...")
	if m.pr.Title != "" {
		pr = s.Width(prWidth).Render(lipgloss.JoinVertical(lipgloss.Left,
			s.Width(prWidth).Render(lipgloss.JoinHorizontal(lipgloss.Top,
				s.Foreground(m.styles.colors.lightColor).Bold(true).Render(m.pr.Repository.NameWithOwner),
				s.Render(" "),
				s.Foreground(m.styles.colors.faintColor).Render(fmt.Sprintf("#%d", m.pr.Number)),
			)),
			s.Width(prWidth).Bold(true).Foreground(m.styles.colors.focusedColor).Render(m.pr.Title),
		))
	}

	return m.styles.headerStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, s.Render(pr), logo))
}

func (m *model) viewFooter() string {
	if m.width == 0 {
		return ""
	}

	failing, successful, skipped := 0, 0, 0
	for _, count := range m.pr.StatusCheckRollup.Contexts.CheckRunCountsByState {
		switch count.State {
		case api.ConclusionFailure:
			failing += count.Count
		case api.ConclusionActionRequired:
		case api.ConclusionCancelled:
		case api.ConclusionNeutral:
		case api.ConclusionSkipped:
			skipped += count.Count
		case api.ConclusionStale:
			skipped += count.Count
		case api.ConclusionStartupFailure:
			failing += count.Count
		case api.ConclusionSuccess:
			successful += count.Count
		case api.ConclusionTimedOut:
			failing += count.Count
		}
	}

	texts := make([]string, 0)
	bg := lipgloss.NewStyle().Background(m.styles.footerStyle.GetBackground())
	if failing > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.errorColor).Render(fmt.Sprintf("%d failing", failing)))
	}
	if successful > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.successColor).Render(
			fmt.Sprintf("%d successful", successful)))
	}
	if skipped > 0 {
		texts = append(texts, bg.Foreground(m.styles.colors.faintColor).Render(fmt.Sprintf("%d skipped", skipped)))
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
	if m.logsViewport.GetContent() != "" {
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

	w := m.getFocusedPaneWidth()
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
	l.Styles.NoItems = l.Styles.NoItems.Foreground(styles.colors.faintColor)
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

	_, rCmds := m.updateRunsList()
	cmds = append(cmds, rCmds...)

	_, jCmds := m.updateJobsList()
	cmds = append(cmds, jCmds...)

	cmds = append(cmds, m.updateStepsList()...)

	return cmds
}

func (m *model) updateRunsList() (*runItem, []tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	if len(m.runsList.VisibleItems()) == 0 {
		return nil, cmds
	}

	run := m.runsList.SelectedItem()
	if run == nil {
		return nil, cmds
	}
	ri, ok := run.(*runItem)
	if !ok {
		return nil, cmds
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

	return ri, cmds
}

func (m *model) updateJobsList() (*jobItem, []tea.Cmd) {
	cmds := make([]tea.Cmd, 0)
	ri := m.getSelectedRunItem()
	if ri == nil {
		return nil, cmds
	}

	jobs := make([]list.Item, 0)
	for _, job := range ri.jobsItems {
		jobs = append(jobs, job)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobs))
	if len(m.jobsList.VisibleItems()) > 0 || m.jobsList.FilterState() == list.FilterApplied {
		m.jobsList.SetShowStatusBar(true)
	} else {
		m.jobsList.SetShowStatusBar(false)
	}

	if m.jobsList.GlobalIndex() >= len(ri.jobsItems) {
		return nil, cmds
	}

	return m.getSelectedJobItem(), cmds
}

func (m *model) updateStepsList() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	steps := make([]list.Item, 0)

	ji := m.getSelectedJobItem()
	if ji != nil {
		for _, step := range ji.steps {
			steps = append(steps, step)
		}
	}

	cmds = append(cmds, m.stepsList.SetItems(steps))
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
	if m.isScrollbarVisible() {
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
	h := m.getMainContentHeight()
	return lipgloss.Place(
		m.logsWidth(),
		h-2,
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

func (m *model) enrichRunWithJobsStepsV2(msg workflowRunStepsFetchedMsg) {
	jobsMap := make(map[string]api.CheckRunWithSteps)
	checks := msg.data.Resource.WorkflowRun.CheckSuite.CheckRuns.Nodes
	for _, check := range checks {
		jobsMap[fmt.Sprintf("%d", check.DatabaseId)] = check
	}

	runs := m.runsList.VisibleItems()

	// find runItem
	var ri *runItem
	for _, run := range runs {
		run := run.(*runItem)
		if run.run.Id == msg.runId {
			ri = run
			break
		}
	}

	if ri == nil {
		log.Error("run not found when trying to enrich with steps", "msg", msg)
		return
	}

	ri.loading = false
	for jobIdx, job := range ri.jobsItems {
		ri.jobsItems[jobIdx].loadingSteps = false
		jobWithSteps, ok := jobsMap[job.job.Id]
		if !ok {
			continue
		}

		for _, step := range jobWithSteps.Steps.Nodes {
			si := NewStepItem(step, jobWithSteps.Url, m.styles)
			ri.jobsItems[jobIdx].steps = append(ri.jobsItems[jobIdx].steps, &si)
		}

	}
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

	cmds = append(cmds, m.logsSpinner.Tick)

	currJob := m.getSelectedJobItem()
	if currJob != nil && !currJob.initiatedLogsFetch {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	} else if currJob == nil {
		log.Error("job changed but current job is nil")
	}

	m.renderJobLogs()
	m.goToErrorInLogs()

	return cmds
}

func (m *model) onStepChanged() {
	job := m.jobsList.SelectedItem()
	step := m.stepsList.SelectedItem()

	if step != nil {
		for i, log := range job.(*jobItem).logs {
			if log.Time.After(step.(*stepItem).step.StartedAt) {
				m.logsViewport.SetYOffset(i - 1)
				break
			}
		}
	}
}

func (m *model) renderJobLogs() {
	currJob := m.jobsList.SelectedItem()
	if currJob == nil || currJob.(*jobItem).loadingLogs {
		m.logsViewport.SetContent("")
	}

	ji, ok := currJob.(*jobItem)
	if !ok {
		return
	}

	if ji.logsErr != nil {
		m.logsViewport.SetContent(ji.logsStderr)
		return
	}

	if len(ji.renderedLogs) != 0 {
		m.logsViewport.SetContentLines(ji.renderedLogs)
		return
	}

	if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun || ji.job.Kind == data.JobKindExternal {
		m.logsViewport.SetContent(ji.renderedText)
		return
	}

	ji.renderedLogs, ji.unstyledLogs = m.renderLogs(ji)
	m.logsViewport.SetContentLines(ji.renderedLogs)
}

func (m *model) logsContentView() string {
	job := m.jobsList.SelectedItem()
	if job == nil {
		return m.styles.faintFgStyle.PaddingLeft(1).Render("Nothing selected...")
	}

	ji := job.(*jobItem)
	if ji.job.Conclusion == api.ConclusionSkipped {
		return m.noLogsView("This job was skipped")
	}

	if ji.loadingLogs || ji.loadingSteps {
		return m.loadingLogsView()
	}

	if ji.job.Bucket == data.CheckBucketCancel {
		return m.fullScreenMessageView(lipgloss.JoinVertical(lipgloss.Center,
			m.styles.faintFgStyle.Render(art.StopSign),
			m.styles.faintFgStyle.Bold(true).Render("This job was cancelled")))
	}

	if ji.job.Bucket == data.CheckBucketPending {
		return m.fullScreenMessageView(lipgloss.NewStyle().Foreground(
			m.styles.colors.warnColor).Render("This job is still running"))
	}

	if ji.logsErr != nil && strings.Contains(ji.logsStderr, "HTTP 410:") {
		return m.fullScreenMessageView("The logs for this run have expired and are no longer available.")
	}

	if m.isScrollbarVisible() {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			m.logsViewport.View(),
			m.scrollbar.(scrollbar.Vertical).View(),
		)
	}
	return m.logsViewport.View()
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
	totalLines := fmt.Sprintf("%d", len(ji.logs))
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
		ln := fmt.Sprintf("%d", i+1)
		ln = strings.Repeat(" ", len(totalLines)-len(ln)) + ln + "  "
		// lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
		// m.styles.lineNumbersStyle.Render(ln), rendered))
		lines = append(lines, rendered)
		unstyledLines = append(unstyledLines, unstyled)
		// logs.WriteString()
		// logs.WriteString()
		// logs.WriteString("\n")
	}
	return lines, unstyledLines
}

func (m *model) getFocusedPaneWidth() int {
	if m.width > smallScreen {
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
		m.logsViewport.SetYOffset(currJob.errorLine)
	} else {
		m.logsViewport.GotoTop()
	}
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

	// TODO: take borders from logsInput view
	vph := h - 2 - lipgloss.Height(m.logsInput.View()) - 2
	m.logsViewport.SetHeight(vph)
	m.scrollbar, _ = m.scrollbar.Update(scrollbar.HeightMsg(vph))
}
