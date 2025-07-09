package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/ui/art"
	"github.com/dlvhdr/gh-enhance/internal/ui/scrollbar"
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
	width         int
	height        int
	prNumber      string
	repo          string
	runsList      list.Model
	jobsList      list.Model
	stepsList     list.Model
	logsViewport  viewport.Model
	scrollbar     tea.Model
	quitting      bool
	focusedPane   focusedPane
	err           error
	runsDelegate  list.DefaultDelegate
	jobsDelegate  list.DefaultDelegate
	stepsDelegate list.DefaultDelegate
}

const (
	unfocusedPaneWidth = 20
	focusedPaneWidth   = 40
)

func NewModel(repo string, number string) model {
	runsList, runsDelegate := newRunsDefaultList()
	runsList.Title = "Runs"
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(focusedPaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList()
	jobsList.Title = "Jobs"
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(unfocusedPaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList()
	stepsList.Title = "Steps"
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(unfocusedPaneWidth)

	vp := viewport.New()
	vp.SoftWrap = false
	vp.KeyMap.Right = rightKey
	vp.KeyMap.Left = leftKey

	sb := scrollbar.NewVertical()
	sb.Style = sb.Style.Inherit(scrollbarStyle)
	sb.ThumbStyle = sb.ThumbStyle.Inherit(scrollbarThumbStyle)
	sb.TrackStyle = sb.TrackStyle.Inherit(scrollbarTrackStyle)

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
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	return m.makeGetPRChecksCmd(m.prNumber)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {

	case workflowRunsFetchedMsg:
		runItems := make([]list.Item, 0)
		for _, run := range msg.runs {
			ri := NewRunItem(run)
			runItems = append(runItems, &ri)
		}

		cmds = append(cmds, m.runsList.SetItems(runItems))
		cmds = append(cmds, m.updateLists()...)

		if len(runItems) > 0 {
			ri := runItems[0].(*runItem)
			cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(ri.run.Id))
			if len(ri.run.Jobs) > 0 {
				m.jobsList.Select(0)
				cmds = append(cmds, m.onJobChanged()...)
			}
		}

	case workflowRunStepsFetchedMsg:
		m.enrichRunWithJobsStepsV2(msg)
		cmds = append(cmds, m.updateLists()...)

	case jobLogsFetchedMsg:
		for _, run := range m.runsList.Items() {
			ri := run.(*runItem)
			for i := range ri.jobsItems {
				if ri.jobsItems[i].job.Id != msg.jobId {
					continue
				}

				ri.jobsItems[i].logs = msg.logs
				ri.jobsItems[i].loadingLogs = false
				currJob := m.jobsList.SelectedItem()
				if currJob != nil && currJob.(*jobItem).job.Id == msg.jobId {
					m.renderJobLogs()
				}

				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case checkRunOutputFetchedMsg:
		run := m.runsList.SelectedItem().(*runItem)
		for i := range run.jobsItems {
			if run.jobsItems[i].job.Id == msg.jobId {
				run.jobsItems[i].renderedText = msg.renderedText
				run.jobsItems[i].loadingLogs = false
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
		m.runsList.SetHeight(msg.Height)
		m.jobsList.SetHeight(msg.Height)
		m.stepsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 2)
		m.logsViewport.SetWidth(m.logsWidth())
		m.scrollbar, cmd = m.scrollbar.Update(scrollbar.HeightMsg(m.logsViewport.Height()))
	case tea.KeyMsg:
		log.Debug("key pressed", "key", msg.String())
		if m.runsList.FilterState() == list.Filtering ||
			m.jobsList.FilterState() == list.Filtering ||
			m.stepsList.FilterState() == list.Filtering {
			break
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

	case errMsg:
		m.err = msg
		return m, nil

	}

	switch m.focusedPane {
	case PaneRuns:
		before := m.runsList.Cursor()
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.runsList.Cursor()
		if before != after {
			cmds = append(cmds, m.updateLists()...)
			cmds = append(cmds, m.onRunChanged()...)
		}
		break
	case PaneJobs:
		before := m.jobsList.Cursor()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.Cursor()
		if before != after {
			cmds = append(cmds, m.onJobChanged()...)
		}
	case PaneSteps:
		before := m.stepsList.Cursor()
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.stepsList.Cursor()
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

	return lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		Height(m.height).
		MaxHeight(m.height).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				paneStyle.Render(m.runsList.View()),
				paneStyle.Render(m.jobsList.View()),
				paneStyle.Render(m.stepsList.View()),
				m.viewLogs(),
			),
		)
}

func (m *model) viewLogs() string {
	title := "Logs"
	if m.focusedPane == PaneLogs {
		title = focusedPaneTitleBarStyle.Render(focusedPaneTitleStyle.Render(title))
	} else {
		title = unfocusedPaneTitleBarStyle.Render(unfocusedPaneTitleStyle.Render(title))
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, m.logsContentView())
}

func (m *model) setFocusedPaneStyles() {
	switch m.focusedPane {
	case PaneRuns:
		setListFocusedStyles(&m.runsList, &m.runsDelegate)
		setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
		break
	case PaneJobs:
		setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		setListFocusedStyles(&m.jobsList, &m.jobsDelegate)
		setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
		break
	case PaneSteps:
		setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		setListFocusedStyles(&m.stepsList, &m.stepsDelegate)
		break
	case PaneLogs:
		setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
	}

	m.logsViewport.SetWidth(m.logsWidth())
}

func setListFocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = focusedPaneTitleStyle
	l.Styles.TitleBar = focusedPaneTitleBarStyle.Width(focusedPaneWidth)
	delegate.Styles.SelectedTitle = focusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = focusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
	delegate.Styles.DimmedDesc = normalItemDescStyle
	l.SetDelegate(delegate)
	l.SetWidth(focusedPaneWidth)
}

func setListUnfocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = unfocusedPaneTitleStyle
	l.Styles.TitleBar = unfocusedPaneTitleBarStyle.Width(unfocusedPaneWidth)
	delegate.Styles.SelectedTitle = unfocusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = unfocusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
	delegate.Styles.DimmedDesc = normalItemDescStyle
	l.SetDelegate(delegate)
	l.SetWidth(unfocusedPaneWidth)
}

func newRunsDefaultList() (list.Model, list.DefaultDelegate) {
	d := newRunItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	return l, d
}

func newJobsDefaultList() (list.Model, list.DefaultDelegate) {
	d := newCheckItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	return l, d
}

func newStepsDefaultList() (list.Model, list.DefaultDelegate) {
	d := newStepItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	return l, d
}

func (m *model) updateLists() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	if len(m.runsList.Items()) == 0 {
		return cmds
	}

	run := m.runsList.SelectedItem().(*runItem)
	jobs := make([]list.Item, 0)
	for _, job := range run.jobsItems {
		jobs = append(jobs, job)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobs))

	if m.jobsList.Cursor() >= len(run.jobsItems) {
		return cmds
	}

	job := m.jobsList.SelectedItem().(*jobItem)
	steps := make([]list.Item, 0)
	for _, step := range job.steps {
		steps = append(steps, step)
	}
	cmds = append(cmds, m.stepsList.SetItems(steps))

	return cmds
}

func (m *model) logsWidth() int {
	borders := 5
	sb := 0
	if m.isScrollbarVisible() {
		sb = lipgloss.Width(m.scrollbar.(scrollbar.Vertical).View())
	}
	return m.width - m.runsList.Width() - m.jobsList.Width() - m.stepsList.Width() - borders - sb
}

func (m *model) loadingLogsView() string {
	return m.fullScreenMessageView("Loading...")
}

func (m *model) fullScreenMessageView(message string) string {
	return lipgloss.Place(
		m.logsWidth(),
		m.height,
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
			emptySetArt += watermarkIllustrationStyle.Render(string(char))
		}
	}

	return lipgloss.Place(
		m.logsWidth(),
		m.logsViewport.Height(),
		lipgloss.Center,
		0.75,
		lipgloss.JoinVertical(
			lipgloss.Center,
			emptySetArt,
			noLogsStyle.Render(message),
		))
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

	runs := m.runsList.Items()

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
			si := NewStepItem(step, jobWithSteps.Url)
			ri.jobsItems[jobIdx].steps = append(ri.jobsItems[jobIdx].steps, &si)
		}

	}
}

func (m *model) onRunChanged() []tea.Cmd {
	runIdx := m.runsList.Cursor()
	cmds := make([]tea.Cmd, 0)
	m.jobsList.Select(0)
	cmds = append(cmds, m.onJobChanged()...)
	newRun := m.runsList.Items()[runIdx].(*runItem)
	if newRun.loading {
		cmds = append(cmds, m.makeFetchWorkflowRunStepsCmd(
			m.runsList.Items()[runIdx].(*runItem).run.Id))
	}
	cmds = append(cmds, m.updateLists()...)
	m.logsViewport.GotoTop()

	return cmds
}

func (m *model) onJobChanged() []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	m.stepsList.Select(0)

	currJob := m.jobsList.SelectedItem()
	if currJob != nil && !currJob.(*jobItem).initiatedLogsFetch {
		cmds = append(cmds, m.makeFetchJobLogsCmd())
	}

	m.renderJobLogs()
	m.logsViewport.GotoTop()
	cmds = append(cmds, m.updateLists()...)
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

	ji := currJob.(*jobItem)
	if ji.renderedLogs != "" {
		m.logsViewport.SetContent(ji.renderedLogs)
		return
	}

	if ji.job.Kind == JobKindCheckRun || ji.job.Kind == JobKindExternal {
		m.logsViewport.SetContent(ji.renderedText)
		return
	}

	// TODO: clean, move to a function, pull logic from parser? Because I go over the lines twice
	logs := strings.Builder{}
	totalLines := fmt.Sprintf("%d", len(ji.logs))
	for i, log := range ji.logs {
		if strings.Contains(log.Log, errorMarker) {
			log.Log = strings.Replace(log.Log, errorMarker, "", 1)
			log.Log = errorBgStyle.Width(m.logsViewport.Width() - scrollbarStyle.GetWidth()).Render(
				lipgloss.JoinHorizontal(lipgloss.Top,
					errorTitleStyle.Render("Error: "), errorStyle.Render(log.Log)))
		}
		ln := fmt.Sprintf("%d", i+1)
		ln = strings.Repeat(" ", len(totalLines)-len(ln)) + ln + "  "
		logs.WriteString(lineNumbersStyle.Render(ln))
		logs.WriteString(log.Log)
		logs.WriteString("\n")
	}
	ji.renderedLogs = logs.String()
	m.logsViewport.SetContent(ji.renderedLogs)
}

func (m *model) logsContentView() string {
	job := m.jobsList.SelectedItem()
	if job == nil {
		return m.loadingLogsView()
	}

	ji := job.(*jobItem)
	if ji.job.Conclusion == api.ConclusionSkipped {
		return m.noLogsView("This job was skipped")
	}

	if ji.loadingLogs || ji.loadingSteps {
		return m.loadingLogsView()
	}

	if m.isScrollbarVisible() {
		return lipgloss.JoinHorizontal(lipgloss.Top,
			m.logsViewport.View(),
			m.scrollbar.(scrollbar.Vertical).View(),
		)
	}
	return m.logsViewport.View()
}
