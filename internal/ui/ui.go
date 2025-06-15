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
	data          []api.CheckRun
	runsList      list.Model
	jobsList      list.Model
	stepsList     list.Model
	logsViewport  viewport.Model
	spinner       spinner.Model
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
	vp.LeftGutterFunc = func(info viewport.GutterContext) string {
		if info.Soft {
			return "     │ "
		}
		if info.Index >= info.TotalLines {
			return "   ~ │ "
		}

		spacing := fmt.Sprintf("%d", info.TotalLines)
		return lineNumbersStyle.Width(len(spacing) + 1).AlignHorizontal(lipgloss.Right).Render(fmt.Sprintf("%d ", info.Index+1))
	}
	vp.SoftWrap = false
	vp.KeyMap.Right = key.Binding{}
	vp.KeyMap.Left = key.Binding{}

	m := model{
		jobsList:      jobsList,
		runsList:      runsList,
		stepsList:     stepsList,
		prNumber:      number,
		repo:          repo,
		spinner:       s,
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
		m.data = msg.runs

		if len(m.data) > 0 {
			run := m.data[0]
			parts := strings.Split(run.Link, "/")
			runId := parts[len(parts)-3]

			// Initialize ids and loading state for each job
			for runIdx, run := range m.data {
				for jobIdx, job := range run.Jobs {
					parts := strings.Split(job.Link, "/")

					m.data[runIdx].Id = parts[len(parts)-3]
					m.data[runIdx].Jobs[jobIdx].Id = parts[len(parts)-1]
					m.data[runIdx].Jobs[jobIdx].Loading = true
				}
			}

			cmds = append(cmds, m.makeFetchRunJobsWithStepsCmd(runId))
		}

		cmds = append(cmds, m.updateLists()...)

	case jobLogsFetchedMsg:
		runIdx := m.runsList.Cursor()
		run := m.data[runIdx]
		for i := range run.Jobs {
			if run.Jobs[i].Id == msg.jobId {
				log.Debug("caching job logs", "jobId", msg.jobId)
				m.data[runIdx].Jobs[i].Logs = msg.logs
				m.data[runIdx].Jobs[i].Loading = false
				cmds = append(cmds, m.updateLists()...)
				break
			}
		}

	case runJobsStepsFetchedMsg:
		jobsMap := make(map[string]api.JobWithSteps)
		for _, job := range msg.jobsWithSteps.Jobs {
			jobsMap[fmt.Sprintf("%d", job.DatabaseId)] = job
		}

		for runIdx, run := range m.data {
			for jobIdx, job := range run.Jobs {
				jobWithSteps, ok := jobsMap[job.Id]
				if !ok {
					continue
				}

				m.data[runIdx].Jobs[jobIdx].Steps = jobWithSteps.Steps
			}
		}

		cmds = append(cmds, m.updateLists()...)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.runsList.SetHeight(msg.Height)
		m.jobsList.SetHeight(msg.Height)
		m.stepsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 1)
		m.logsViewport.SetWidth(m.width - m.runsList.Width() - m.jobsList.Width() - m.stepsList.Width() - 4)
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

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	switch m.focusedPane {
	case PaneRuns:
		before := m.runsList.Cursor()
		m.runsList, cmd = m.runsList.Update(msg)
		after := m.runsList.Cursor()
		if before != after {
			m.jobsList.Select(0)
			m.stepsList.Select(0)
			cmds = append(cmds, m.makeFetchJobLogsCmd())
			cmds = append(cmds, m.makeFetchRunJobsWithStepsCmd(m.data[after].Id))
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
		m.stepsList, cmd = m.stepsList.Update(msg)
		cmds = append(cmds, cmd)

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

	currCheck := m.jobsList.SelectedItem()
	if currCheck != nil {
		m.logsViewport.SetContent(currCheck.(jobItem).logs)
	}

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

	check := m.jobsList.SelectedItem()
	if check == nil || check.(jobItem).loading {
		content = "loading..."
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

	m.logsViewport.SetWidth(m.width - m.runsList.Width() - m.jobsList.Width() - m.stepsList.Width() - 4)
}

func setListFocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = focusedPaneTitleStyle
	l.Styles.TitleBar = focusedPaneTitleBarStyle.Width(focusedPaneWidth)
	delegate.Styles.SelectedTitle = focusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = focusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
	l.SetDelegate(delegate)
	l.SetWidth(focusedPaneWidth)
}

func setListUnfocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = unfocusedPaneTitleStyle
	l.Styles.TitleBar = unfocusedPaneTitleBarStyle.Width(unfocusedPaneWidth)
	delegate.Styles.SelectedTitle = unfocusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = unfocusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
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
	runItems := make([]list.Item, 0)
	for _, run := range m.data {
		it := runItem{title: run.Name, description: run.Link, workflow: run.Workflow}
		runItems = append(runItems, it)
		cmds = append(cmds, m.runsList.SetItems(runItems))
	}

	if m.runsList.Cursor() >= len(m.data) {
		return cmds
	}

	run := m.data[m.runsList.Cursor()]
	jobItems := make([]list.Item, 0)
	for _, job := range run.Jobs {
		parts := strings.Split(job.Link, "/")
		id := parts[len(parts)-1]
		jobItem := jobItem{
			title:       job.Name,
			description: id,
			workflow:    job.Workflow,
			id:          id,
			logs:        job.Logs,
			loading:     job.Loading,
			state:       job.State,
		}
		jobItems = append(jobItems, jobItem)
	}
	cmds = append(cmds, m.jobsList.SetItems(jobItems))

	if m.jobsList.Cursor() >= len(run.Jobs) {
		return cmds
	}

	job := run.Jobs[m.jobsList.Cursor()]

	stepItems := make([]list.Item, 0)
	for _, step := range job.Steps {
		stepItem := stepItem{
			title:       step.Name,
			description: step.StartedAt,
			state:       step.Status,
		}
		stepItems = append(stepItems, stepItem)
	}
	cmds = append(cmds, m.stepsList.SetItems(stepItems))

	return cmds
}
