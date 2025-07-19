package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/paginator"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
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
	scrollbar     tea.Model
	quitting      bool
	focusedPane   focusedPane
	err           error
	runsDelegate  list.ItemDelegate
	jobsDelegate  list.ItemDelegate
	stepsDelegate list.ItemDelegate
	styles        styles
	logsSpinner   spinner.Model
}

const (
	unfocusedPaneWidth = 20
	focusedPaneWidth   = 40
)

func NewModel(repo string, number string) model {
	tint.NewDefaultRegistry()
	tint.SetTint(tint.TintTokyoNight)

	s := makeStyles()

	runsList, runsDelegate := newRunsDefaultList(s)
	runsList.Title = makePill(ListSymbol+" Runs", s.focusedPaneTitleStyle,
		s.colors.focusedColor)
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(focusedPaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList(s)
	jobsList.Title = makePill(ListSymbol+" Jobs", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(unfocusedPaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList(s)
	stepsList.Title = makePill(ListSymbol+" Steps", s.unfocusedPaneTitleStyle,
		s.colors.unfocusedColor)
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(unfocusedPaneWidth)

	vp := viewport.New()
	vp.SoftWrap = false
	vp.KeyMap.Right = rightKey
	vp.KeyMap.Left = leftKey

	sb := scrollbar.NewVertical()
	sb.Style = sb.Style.Inherit(s.scrollbarStyle)
	sb.ThumbStyle = sb.ThumbStyle.Inherit(s.scrollbarThumbStyle)
	sb.TrackStyle = sb.TrackStyle.Inherit(s.scrollbarTrackStyle)

	ls := spinner.New(spinner.WithSpinner(LogsFrames))

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
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	// return nil
	return tea.Batch(m.runsList.StartSpinner(), m.logsSpinner.Tick, m.jobsList.StartSpinner(), m.makeGetPRChecksCmd(m.prNumber))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {

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
		m.width = msg.Width
		m.height = msg.Height
		lHeight := msg.Height - headerHeight - footerHeight
		m.runsList.SetHeight(lHeight)
		m.jobsList.SetHeight(lHeight)
		m.stepsList.SetHeight(lHeight)
		m.logsViewport.SetHeight(lHeight - 2)
		m.logsViewport.SetWidth(m.logsWidth())
		m.scrollbar, cmd = m.scrollbar.Update(scrollbar.HeightMsg(m.logsViewport.Height()))
	case tea.KeyMsg:
		log.Debug("key pressed", "key", msg.String())
		if m.runsList.FilterState() == list.Filtering ||
			m.jobsList.FilterState() == list.Filtering ||
			m.stepsList.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, openPR) && m.pr.Url != "" {
			cmds = append(cmds, makeOpenUrlCmd(m.pr.Url))
		}

		if key.Matches(msg, nextPaneKey) {
			m.focusedPane = min(PaneLogs, m.focusedPane+1)
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, prevPaneKey) {
			m.focusedPane = max(PaneRuns, m.focusedPane-1)
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, quitKeys) {
			m.quitting = true
			return m, tea.Quit
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
		if msg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(msg, gotoBottomKey) {
				m.logsViewport.GotoBottom()
			}

			if key.Matches(msg, gotoTopKey) {
				m.logsViewport.GotoTop()
			}
		}
		m.logsViewport, cmd = m.logsViewport.Update(msg)
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

	steps := ""
	if m.shouldShowSteps() {
		steps = m.styles.paneStyle.Render(m.stepsList.View())
	}

	rootStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height)

	header := m.viewHeader()
	footer := m.viewFooter()

	return rootStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.styles.paneStyle.Render(m.runsList.View()),
			m.styles.paneStyle.Render(m.jobsList.View()),
			steps,
			m.viewLogs(),
		),
		footer,
	))
}

func (m *model) viewHeader() string {
	pr := lipgloss.NewStyle().Foreground(m.styles.colors.faintColor).Render("Loading...")
	if m.pr.Title != "" {
		pr = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Foreground(m.styles.colors.lightColor).Bold(true).Render(m.pr.Repository.NameWithOwner),
				" ",
				lipgloss.NewStyle().Foreground(m.styles.colors.faintColor).Render(fmt.Sprintf("#%d", m.pr.Number)),
			),
			lipgloss.NewStyle().Bold(true).Render(m.pr.Title),
		)
	}
	logo := lipgloss.JoinHorizontal(lipgloss.Bottom,
		m.styles.logoStyle.Render(Logo),
		" ",
		lipgloss.NewStyle().Foreground(m.styles.colors.faintColor).Render("v0.1.0〓"))
	w := m.width - lipgloss.Width(logo) - m.styles.headerStyle.GetHorizontalFrameSize()
	return m.styles.headerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(w).Render(pr), logo))
}

func (m *model) viewFooter() string {
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

	return m.styles.footerStyle.Width(m.width).Render(strings.Join(texts, bg.Render(", ")))
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
	w := m.logsWidth() - 1
	if m.focusedPane == PaneLogs {
		title = makePill(title, m.styles.focusedPaneTitleStyle, m.styles.colors.focusedColor)
		s := m.styles.focusedPaneTitleBarStyle.Width(w)
		title = s.Render(title)
	} else {
		title = makePill(title, m.styles.unfocusedPaneTitleStyle, m.styles.colors.unfocusedColor)
		s := m.styles.unfocusedPaneTitleBarStyle.Width(w)
		title = s.Render(title)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, m.logsContentView())
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

	m.logsViewport.SetWidth(m.logsWidth())
}

func (m *model) setListFocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	l.Styles.Title = m.styles.focusedPaneTitleStyle
	title := ansi.Strip(l.Title)
	title, _ = strings.CutPrefix(title, "")
	title, _ = strings.CutSuffix(title, "")
	l.Title = makePill(title, l.Styles.Title, m.styles.colors.focusedColor)
	l.Styles.TitleBar = m.styles.focusedPaneTitleBarStyle
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(focusedPaneWidth)
	l.SetDelegate(*delegate)
	l.SetWidth(focusedPaneWidth)
}

func (m *model) setListUnfocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	l.Styles.Title = m.styles.unfocusedPaneTitleStyle
	title := ansi.Strip(l.Title)
	title, _ = strings.CutPrefix(title, "")
	title, _ = strings.CutSuffix(title, "")
	l.Title = makePill(title, l.Styles.Title, m.styles.colors.unfocusedColor)
	l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(unfocusedPaneWidth)
	l.SetDelegate(*delegate)
	l.SetWidth(unfocusedPaneWidth)
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

	if len(m.runsList.VisibleItems()) == 0 {
		return cmds
	}

	run := m.runsList.SelectedItem()
	if run == nil {
		return nil
	}
	ri, ok := run.(*runItem)
	if !ok {
		return nil
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
		return cmds
	}

	job := m.jobsList.SelectedItem()
	steps := make([]list.Item, 0)
	if job != nil {
		ji, ok := job.(*jobItem)
		if ok {
			for _, step := range ji.steps {
				steps = append(steps, step)
			}
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

func (m *model) logsWidth() int {
	borders := 2
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
	return lipgloss.Place(
		m.logsWidth(),
		m.height-headerHeight-footerHeight-2, // -2 for logs title
		lipgloss.Center,
		0.75,
		message,
	)
}

func (m *model) noLogsView(message string) string {
	emptySetArt := ""
	for _, char := range art.EmptySet {
		if char == '╱' {
			emptySetArt += lipgloss.NewStyle().Foreground(lipgloss.Red).Render("╱")
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
	m.stepsList.ResetSelected()
	m.stepsList.ResetFilter()

	cmds = append(cmds, m.logsSpinner.Tick)

	currJob := m.jobsList.SelectedItem()
	if currJob != nil && !currJob.(*jobItem).initiatedLogsFetch {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	} else {
		log.Error("job changed but current job is nil", "currJob", currJob)
	}

	m.renderJobLogs()
	m.logsViewport.GotoTop()
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

	if ji.renderedLogs != "" {
		m.logsViewport.SetContent(ji.renderedLogs)
		return
	}

	if ji.job.Kind == data.JobKindCheckRun || ji.job.Kind == data.JobKindExternal {
		m.logsViewport.SetContent(ji.renderedText)
		return
	}

	ji.renderedLogs = m.renderLogs(ji)
	m.logsViewport.SetContent(ji.renderedLogs)
}

func (m *model) logsContentView() string {
	job := m.jobsList.SelectedItem()
	if job == nil {
		return "Nothing selected..."
	}

	ji := job.(*jobItem)
	if ji.job.Conclusion == api.ConclusionSkipped {
		return m.noLogsView("This job was skipped")
	}

	if ji.loadingLogs || ji.loadingSteps {
		return m.loadingLogsView()
	}

	if ji.job.Bucket == data.CheckBucketCancel {
		return m.fullScreenMessageView("This job was cancelled")
	}

	if ji.job.Bucket == data.CheckBucketPending {
		return m.fullScreenMessageView("This job is still running")
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

func (m *model) renderLogs(ji *jobItem) string {
	defer utils.TimeTrack(time.Now(), "rendering logs")
	logs := strings.Builder{}
	totalLines := fmt.Sprintf("%d", len(ji.logs))
	w := m.logsViewport.Width() - m.styles.scrollbarStyle.GetWidth()
	expand := ExpandSymbol + " "
	for i, log := range ji.logs {
		rendered := log.Log
		switch log.Kind {
		case data.LogKindError:
			rendered = strings.Replace(rendered, parser.ErrorMarker, "", 1)
			rendered = m.styles.errorBgStyle.Width(w).Render(
				lipgloss.JoinHorizontal(lipgloss.Top,
					m.styles.errorTitleStyle.Render("Error: "), m.styles.errorStyle.Render(rendered)))
		case data.LogKindCommand:
			rendered = strings.Replace(rendered, parser.CommandMarker, "", 1)
			rendered = m.styles.commandStyle.Render(rendered)
		case data.LogKindGroupStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			rendered = m.styles.groupStartMarkerStyle.Render(rendered)
		case data.LogKindJobCleanup:
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepStart:
			rendered = strings.Replace(rendered, parser.GroupStartMarker, expand, 1)
			rendered = m.styles.stepStartMarkerStyle.Render(rendered)
		case data.LogKindStepNone:
			sep := ""
			if log.Depth > 0 {
				sep = m.styles.separatorStyle.Render(strings.Repeat(
					fmt.Sprintf("%s  ", Separator), log.Depth))
			}
			rendered = sep + rendered
		}
		ln := fmt.Sprintf("%d", i+1)
		ln = strings.Repeat(" ", len(totalLines)-len(ln)) + ln + "  "
		logs.WriteString(m.styles.lineNumbersStyle.Render(ln))
		logs.WriteString(rendered)
		logs.WriteString("\n")
	}
	return logs.String()
}
