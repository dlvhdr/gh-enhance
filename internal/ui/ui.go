package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type errMsg error

type model struct {
	width          int
	height         int
	prNumber       string
	repo           string
	checks         []api.Check
	runs           []api.Run
	checksList     list.Model
	logsViewport   viewport.Model
	runsList       list.Model
	spinner        spinner.Model
	quitting       bool
	focusedPane    int
	err            error
	runsDelegate   list.DefaultDelegate
	checksDelegate list.DefaultDelegate
}

const (
	firstPaneWidth  = 20
	secondPaneWidth = 40
)

func NewModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	runsList, runsDelegate := newDefaultList()
	runsList.Title = "Checks"
	runsList.SetStatusBarItemName("check", "checks")
	runsList.SetWidth(firstPaneWidth)

	checksList, checksDelegate := newDefaultList()
	checksList.Title = "Jobs"
	runsList.SetStatusBarItemName("job", "jobs")
	checksList.SetWidth(secondPaneWidth)

	vp := viewport.New()

	m := model{
		checksList:     checksList,
		runsList:       runsList,
		prNumber:       "34454",
		repo:           "neovim/neovim",
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
		m.checks = msg.checks
		m.runs = msg.runs
		runItems := make([]list.Item, 0)
		for _, run := range m.runs {
			it := item{title: run.Name, description: run.Link, workflow: run.Workflow}
			runItems = append(runItems, it)
		}

		cmd = m.runsList.SetItems(runItems)
		cmds = append(cmds, cmd)

		cmds = append(cmds, m.updateChecksListItems())
		// cmds = append(cmds, m.makeFetchJobLogsCmd(job))

	case jobLogsFetchedMsg:
		m.logsViewport.SetContent(msg.logs)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.checksList.SetHeight(msg.Height)
		m.runsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 1)
		m.logsViewport.Style.Width(m.logsViewport.Width())
		m.logsViewport.SetWidth(m.width - m.runsList.Width() - m.checksList.Width() - 4)
		m.logsViewport.SoftWrap = true
	case tea.KeyMsg:
		log.Debug("key pressed", "key", msg.String())
		if m.runsList.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, nextPaneKey) {
			m.focusedPane = min(1, m.focusedPane+1)
			m.setFocusedPaneStyles()
		}

		if key.Matches(msg, prevPaneKey) {
			m.focusedPane = max(0, m.focusedPane-1)
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

	if m.focusedPane == 0 {
		before := m.runsList.Cursor()
		m.runsList, cmd = m.runsList.Update(msg)
		after := m.runsList.Cursor()
		cmds = append(cmds, cmd)
		m.updateChecksListItems()
		if before != after {
			m.checksList.Select(0)
		}
	} else if m.focusedPane == 1 {
		m.checksList, cmd = m.checksList.Update(msg)
		cmds = append(cmds, cmd)
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
				paneStyle.Render(m.checksList.View()),
				m.logsViewport.View(),
			),
		)
}

func (m *model) setFocusedPaneStyles() {
	var focusedList *list.Model
	var unfocusedList *list.Model
	var focusedDelegate *list.DefaultDelegate
	var unfocusedDelegate *list.DefaultDelegate

	if m.focusedPane == 0 {
		focusedList = &m.runsList
		unfocusedList = &m.checksList
		focusedDelegate = &m.runsDelegate
		unfocusedDelegate = &m.checksDelegate
	} else {
		focusedList = &m.checksList
		unfocusedList = &m.runsList
		focusedDelegate = &m.checksDelegate
		unfocusedDelegate = &m.runsDelegate
	}

	focusedList.Styles.Title = focusedPaneTitleStyle
	focusedList.Styles.TitleBar = focusedPaneTitleBarStyle
	focusedDelegate.Styles.SelectedTitle = focusedPaneItemTitleStyle
	focusedDelegate.Styles.SelectedDesc = focusedPaneItemDescStyle
	focusedDelegate.Styles.NormalDesc = normalItemDescStyle
	focusedList.SetDelegate(focusedDelegate)

	unfocusedList.Styles.Title = unfocusedPaneTitleStyle
	unfocusedList.Styles.TitleBar = unfocusedPaneTitleBarStyle
	unfocusedDelegate.Styles.SelectedTitle = unfocusedPaneItemTitleStyle
	unfocusedDelegate.Styles.SelectedDesc = unfocusedPaneItemDescStyle
	unfocusedDelegate.Styles.NormalDesc = normalItemDescStyle
	unfocusedList.SetDelegate(unfocusedDelegate)

	m.runsList.Styles.TitleBar = m.runsList.Styles.TitleBar.Width(firstPaneWidth + 1)
	m.checksList.Styles.TitleBar = m.checksList.Styles.TitleBar.Width(secondPaneWidth + 1)
}

func newDefaultList() (list.Model, list.DefaultDelegate) {
	d := newItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)
	l.KeyMap.NextPage = key.Binding{}
	l.KeyMap.PrevPage = key.Binding{}
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)

	return l, d
}

func (m *model) updateChecksListItems() tea.Cmd {
	checkItems := make([]list.Item, 0)
	for _, check := range m.checks {
		if check.Workflow != (m.runsList.SelectedItem().(item)).workflow {
			continue
		}

		it := item{title: check.Name, description: check.Workflow}
		checkItems = append(checkItems, it)
	}

	return m.checksList.SetItems(checkItems)
}
