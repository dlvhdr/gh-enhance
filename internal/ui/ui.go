package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/ui/art"
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
	spinners      []spinner.Model
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

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
	vp.KeyMap.Right = key.Binding{}
	vp.KeyMap.Left = key.Binding{}

	m := model{
		jobsList:      jobsList,
		runsList:      runsList,
		stepsList:     stepsList,
		prNumber:      number,
		repo:          repo,
		spinners:      []spinner.Model{},
		runsDelegate:  runsDelegate,
		jobsDelegate:  jobsDelegate,
		stepsDelegate: stepsDelegate,
		logsViewport:  vp,
	}
	m.setFocusedPaneStyles()
	return m
}

func (m model) Init() tea.Cmd {
	return m.makeGetPrChecksCmd(m.prNumber)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)

	log.Debug("got msg", "type", fmt.Sprintf("%T", msg))
	switch msg := msg.(type) {

	case runsFetchedMsg:
		runItems := make([]list.Item, 0)
		for _, run := range msg.runs {
			ri := NewRunItem(run)
			runItems = append(runItems, &ri)
		}

		cmds = append(cmds, m.runsList.SetItems(runItems))
		if len(runItems) > 0 {
			cmds = append(cmds, m.makeFetchRunJobsWithStepsCmd(runItems[0].(*runItem).run.Id))
		}

		cmds = append(cmds, m.updateLists()...)

	case jobLogsFetchedMsg:
		run := m.runsList.SelectedItem().(*runItem)
		for i := range run.jobs {
			if run.jobs[i].job.Id == msg.jobId {
				run.jobs[i].logs = msg.logs
				run.jobs[i].loadingLogs = false
				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case checkRunOutputFetchedMsg:
		run := m.runsList.SelectedItem().(*runItem)
		for i := range run.jobs {
			if run.jobs[i].job.Id == msg.jobId {
				run.jobs[i].summary = msg.summary
				run.jobs[i].title = msg.summary
				run.jobs[i].kind = "check-run"
				run.jobs[i].loadingLogs = false
				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case runJobsStepsFetchedMsg:
		jobsMap := make(map[string]api.JobWithSteps)
		for _, job := range msg.jobsWithSteps.Jobs {
			jobsMap[fmt.Sprintf("%d", job.DatabaseId)] = job
		}

		runs := m.runsList.Items()
		for _, run := range runs {
			run := run.(*runItem)
			if run.run.Id == msg.runId {
				run.loading = false
			}
			for jobIdx, job := range run.jobs {
				run.jobs[jobIdx].loadingSteps = false
				jobWithSteps, ok := jobsMap[job.job.Id]
				if !ok {
					continue
				}

				for _, step := range jobWithSteps.Steps {
					si := NewStepItem(step, jobWithSteps.Url)
					run.jobs[jobIdx].steps = append(run.jobs[jobIdx].steps, &si)
				}

			}
		}

		cmds = append(cmds, m.makeFetchJobLogsCmd())
		cmds = append(cmds, m.updateLists()...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.runsList.SetHeight(msg.Height)
		m.jobsList.SetHeight(msg.Height)
		m.stepsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 1)
		m.logsViewport.SetWidth(m.logsWidth())
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
			m.jobsList.Select(0)
			m.stepsList.Select(0)
			cmds = append(cmds, m.makeFetchJobLogsCmd())
			newRun := m.runsList.Items()[after].(*runItem)
			if newRun.loading {
				cmds = append(cmds, m.makeFetchRunJobsWithStepsCmd(m.runsList.Items()[after].(*runItem).run.Id))
			}
			cmds = append(cmds, m.updateLists()...)
		}
		break
	case PaneJobs:
		before := m.jobsList.Cursor()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.Cursor()
		if before != after {
			m.stepsList.Select(0)
			cmds = append(cmds, m.makeFetchJobLogsCmd())
			cmds = append(cmds, m.updateLists()...)
		}
	case PaneSteps:
		before := m.stepsList.Cursor()
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.stepsList.Cursor()
		if before != after {
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

	case PaneLogs:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if key.Matches(msg, gotoBottomKey) {
				m.logsViewport.GotoBottom()
			}

			if key.Matches(msg, gotoTopKey) {
				m.logsViewport.GotoTop()
			}
		}
		log.Debug("updating viewport", "msg", msg, "offset", m.logsViewport.YOffset)
		m.logsViewport, cmd = m.logsViewport.Update(msg)
		log.Debug("after updating viewport", "msg", msg, "offset", m.logsViewport.YOffset)
		cmds = append(cmds, cmd)
	}

	currJob := m.jobsList.SelectedItem()
	currStep := m.stepsList.SelectedItem()
	if currJob != nil && currStep != nil && len(currJob.(*jobItem).logs) > m.stepsList.Cursor() {
		logs := strings.Builder{}
		totalLines := fmt.Sprintf("%d", len(currJob.(*jobItem).logs))
		for i, log := range currJob.(*jobItem).logs {
			if strings.Contains(log.Log, errorMarker) {
				log.Log = strings.Replace(log.Log, errorMarker, "", 1)
				log.Log = errorBgStyle.Width(m.logsViewport.Width()).Render(lipgloss.JoinHorizontal(lipgloss.Top, errorTitleStyle.Render("Error: "), errorStyle.Render(log.Log)))
			}
			ln := fmt.Sprintf("%d", i+1)
			ln = ln + strings.Repeat(" ", len(totalLines)-len(ln)) + "  "
			logs.WriteString(lineNumbersStyle.Render(ln))
			logs.WriteString(log.Log)
			logs.WriteString("\n")
		}
		m.logsViewport.SetContent(logs.String())
		// m.logsViewport.GotoTop()
	} else if currJob != nil && currJob.(*jobItem).kind == "check-run" && !currJob.(*jobItem).loadingLogs {
		m.logsViewport.SetContent(currJob.(*jobItem).summary)
		// m.logsViewport.GotoTop()
	}

	m.setFocusedPaneStyles()
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
	var content string
	title := "Logs"

	if m.focusedPane == PaneLogs {
		title = focusedPaneTitleBarStyle.Render(focusedPaneTitleStyle.Render(title))
	} else {
		title = unfocusedPaneTitleBarStyle.Render(unfocusedPaneTitleStyle.Render(title))
	}

	job := m.jobsList.SelectedItem()
	if job == nil || job.(*jobItem).loadingLogs || job.(*jobItem).loadingSteps {
		content = lipgloss.Place(
			m.logsWidth(),
			m.height,
			lipgloss.Center,
			0.75,
			"Loading...",
			// m.spinner.View(),
		)
	} else {
		content = m.logsViewport.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, content)
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
	for _, job := range run.jobs {
		jobs = append(jobs, job)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobs))

	if m.jobsList.Cursor() >= len(run.jobs) {
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
	return m.width - m.runsList.Width() - m.jobsList.Width() - m.stepsList.Width() - borders
}

func (m *model) noLogsView() string {
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
		m.height,
		lipgloss.Center,
		0.75,
		lipgloss.JoinVertical(
			lipgloss.Center,
			emptySetArt,
			noLogsStyle.Render("This job doesn't have any logs"),
		))
}
