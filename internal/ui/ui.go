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
	width          int
	height         int
	prNumber       string
	repo           string
	runsList       list.Model
	jobsList       list.Model
	stepsList      list.Model
	logsViewport   viewport.Model
	spinner        spinner.Model
	quitting       bool
	focusedPane    focusedPane
	err            error
	runsDelegate   list.DefaultDelegate
	checksDelegate list.DefaultDelegate
}

const (
	firstPaneWidth  = 20
	secondPaneWidth = 40
)

func NewModel(repo string, number string) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	runsList, runsDelegate := newRunsDefaultList()
	runsList.Title = "Runs"
	runsList.SetStatusBarItemName("run", "runs")
	runsList.SetWidth(firstPaneWidth)

	checksList, checksDelegate := newChecksDefaultList()
	checksList.Title = "Jobs"
	checksList.SetStatusBarItemName("job", "jobs")
	checksList.SetWidth(secondPaneWidth)

	stepsList, checksDelegate := newChecksDefaultList()
	stepsList.Title = "Steps"
	stepsList.SetStatusBarItemName("step", "steps")
	stepsList.SetWidth(secondPaneWidth)

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
		jobsList:       checksList,
		runsList:       runsList,
		prNumber:       number,
		repo:           repo,
		spinner:        s,
		runsDelegate:   runsDelegate,
		checksDelegate: checksDelegate,
		logsViewport:   vp,
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
			jobItems := make([]jobItem, 0)
			for _, job := range run.Jobs {
				parts := strings.Split(job.Link, "/")
				id := parts[len(parts)-1]
				jobItem := jobItem{
					title:       job.Name,
					description: id,
					workflow:    job.Workflow,
					id:          id,
					logs:        "",
					loading:     true,
					state:       job.State,
				}
				jobItems = append(jobItems, jobItem)
			}

			it := runItem{title: run.Name, description: run.Link, workflow: run.Workflow, jobs: jobItems}
			runItems = append(runItems, it)
		}

		cmd = m.runsList.SetItems(runItems)
		cmds = append(cmds, cmd)

		cmds = append(cmds, m.updateJobsListItems())
		cmds = append(cmds, m.makeFetchJobStepsAndLogsCmd())

	case jobLogsFetchedMsg:
		run := m.runsList.SelectedItem().(runItem)
		for i := range run.jobs {
			if run.jobs[i].id == msg.jobId {
				log.Debug("caching job logs", "jobId", msg.jobId)
				run.jobs[i].logs = msg.logs
				run.jobs[i].loading = false
				cmd := m.updateJobsListItems()
				cmds = append(cmds, cmd)
				break
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.jobsList.SetHeight(msg.Height)
		m.runsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 1)
		m.logsViewport.SetWidth(m.width - m.runsList.Width() - m.jobsList.Width() - 4)
	case tea.KeyMsg:
		log.Debug("key pressed", "key", msg.String())
		if m.runsList.FilterState() == list.Filtering {
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
			cmd := m.updateJobsListItems()
			cmds = append(cmds, cmd)
			m.jobsList.Select(0)
			cmds = append(cmds, m.makeFetchJobStepsAndLogsCmd())
		}
		break
	case PaneJobs:
		before := m.jobsList.Cursor()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.Cursor()
		if before != after {
			cmds = append(cmds, m.makeFetchJobStepsAndLogsCmd())
		}
	case PaneSteps:
		before := m.jobsList.Cursor()
		m.jobsList, cmd = m.jobsList.Update(msg)
		cmds = append(cmds, cmd)
		after := m.jobsList.Cursor()
		if before != after {
			cmds = append(cmds, m.makeFetchJobStepsAndLogsCmd())
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
		setListUnfocusedStyles(&m.jobsList, &m.checksDelegate)
		break
	case PaneJobs:
		setListFocusedStyles(&m.jobsList, &m.checksDelegate)
		setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
		break
	case PaneLogs:
		setListUnfocusedStyles(&m.jobsList, &m.checksDelegate)
		setListUnfocusedStyles(&m.runsList, &m.runsDelegate)
	}

	m.runsList.Styles.TitleBar = m.runsList.Styles.TitleBar.Width(firstPaneWidth + 1)
	m.jobsList.Styles.TitleBar = m.jobsList.Styles.TitleBar.Width(secondPaneWidth + 1)
}

func setListFocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = focusedPaneTitleStyle
	l.Styles.TitleBar = focusedPaneTitleBarStyle
	delegate.Styles.SelectedTitle = focusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = focusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
	l.SetDelegate(delegate)
}

func setListUnfocusedStyles(l *list.Model, delegate *list.DefaultDelegate) {
	l.Styles.Title = unfocusedPaneTitleStyle
	l.Styles.TitleBar = unfocusedPaneTitleBarStyle
	delegate.Styles.SelectedTitle = unfocusedPaneItemTitleStyle
	delegate.Styles.SelectedDesc = unfocusedPaneItemDescStyle
	delegate.Styles.NormalDesc = normalItemDescStyle
	l.SetDelegate(delegate)
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

func newChecksDefaultList() (list.Model, list.DefaultDelegate) {
	d := newCheckItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	return l, d
}

func (m *model) updateJobsListItems() tea.Cmd {
	run := m.runsList.SelectedItem().(runItem)
	items := make([]list.Item, 0)
	for _, item := range run.jobs {
		items = append(items, item)
	}
	return m.jobsList.SetItems(items)
}
