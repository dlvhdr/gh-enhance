package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
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
	runsDelegate  list.ItemDelegate
	jobsDelegate  list.ItemDelegate
	stepsDelegate list.ItemDelegate
	styles        styles
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
	runsList.Title = ListSymbol + " Runs"
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(focusedPaneWidth)

	jobsList, jobsDelegate := newJobsDefaultList(s)
	jobsList.Title = ListSymbol + " Jobs"
	jobsList.SetStatusBarItemName("job", "jobs")
	jobsList.SetWidth(unfocusedPaneWidth)

	stepsList, stepsDelegate := newStepsDefaultList(s)
	stepsList.Title = ListSymbol + " Steps"
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
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.runsList.StartSpinner(), m.jobsList.StartSpinner(), m.makeGetPRChecksCmd(m.prNumber))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {

	case workflowRunsFetchedMsg:
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
				m.jobsList.Select(0)
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
			ji.loadingLogs = false
			currJob := m.jobsList.SelectedItem()
			if currJob != nil && currJob.(*jobItem).job.Id == msg.jobId {
				m.renderJobLogs()
			}

			cmds = append(cmds, m.updateLists()...)
			break
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

	case spinner.TickMsg:
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
				m.styles.paneStyle.Render(m.runsList.View()),
				m.styles.paneStyle.Render(m.jobsList.View()),
				m.styles.paneStyle.Render(m.stepsList.View()),
				m.viewLogs(),
			),
		)
}

func (m *model) viewLogs() string {
	title := "⏺︎ Full Job Logs"
	if m.focusedPane == PaneLogs {
		title = m.styles.focusedPaneTitleBarStyle.Render(
			m.styles.focusedPaneTitleStyle.Width(m.logsWidth() - 1).Render(title))
	} else {
		title = m.styles.unfocusedPaneTitleBarStyle.Render(
			m.styles.unfocusedPaneTitleStyle.Width(m.logsWidth() - 1).Render(title))
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
		break
	case PaneJobs:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = true
		m.stepsDelegate.(*stepsDelegate).focused = false
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListFocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListUnfocusedStyles(&m.stepsList, &m.stepsDelegate)
		break
	case PaneSteps:
		m.runsDelegate.(*runsDelegate).focused = false
		m.jobsDelegate.(*jobsDelegate).focused = false
		m.stepsDelegate.(*stepsDelegate).focused = true
		m.setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		m.setListUnfocusedStyles(&m.jobsList, &m.jobsDelegate)
		m.setListFocusedStyles(&m.stepsList, &m.stepsDelegate)
		break
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
	l.Styles.TitleBar = m.styles.focusedPaneTitleBarStyle
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(focusedPaneWidth)
	l.SetDelegate(*delegate)
	l.SetWidth(focusedPaneWidth)
}

func (m *model) setListUnfocusedStyles(l *list.Model, delegate *list.ItemDelegate) {
	l.Styles.Title = m.styles.unfocusedPaneTitleStyle
	l.Styles.TitleBar = m.styles.unfocusedPaneTitleBarStyle
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1).Width(unfocusedPaneWidth)
	l.SetDelegate(*delegate)
	l.SetWidth(unfocusedPaneWidth)
}

func newRunsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newRunItemDelegate(styles)
	return newList(d), d
}

func newJobsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newJobItemDelegate(styles)
	return newList(d), d
}

func newStepsDefaultList(styles styles) (list.Model, list.ItemDelegate) {
	d := newStepItemDelegate(styles)
	return newList(d), d
}

func newList(delegate list.ItemDelegate) list.Model {
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Styles.StatusBar = l.Styles.StatusBar.PaddingLeft(1)
	l.Styles.Spinner = lipgloss.NewStyle().Width(5).Background(lipgloss.Red)
	l.SetSpinner(spinner.Dot)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.StartSpinner()
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	return l
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
	borders := 3
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
			emptySetArt += m.styles.watermarkIllustrationStyle.Render(string(char))
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
			m.styles.noLogsStyle.Render(message),
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
			si := NewStepItem(step, jobWithSteps.Url, m.styles)
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
