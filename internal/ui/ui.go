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

	runsDelegate := newItemDelegate()
	runsList := list.New([]list.Item{}, runsDelegate, 0, 0)
	runsList.Title = "Checks"
	runsList.Styles.TitleBar = focusedPaneTitleStyle
	runsList.SetStatusBarItemName("check", "checks")
	runsList.SetSize(firstPaneWidth, 0)
	runsList.KeyMap.NextPage = key.Binding{}
	runsList.KeyMap.PrevPage = key.Binding{}

	checksDelegate := newItemDelegate()
	checksList := list.New([]list.Item{}, checksDelegate, 0, 0)
	checksList.Styles.TitleBar = unfocusedPaneTitleStyle
	checksList.Title = "Jobs"
	checksList.SetSize(secondPaneWidth, 0)
	checksList.KeyMap.NextPage = key.Binding{}
	checksList.KeyMap.PrevPage = key.Binding{}

	vp := viewport.New()

	m := model{
		checksList:     checksList,
		runsList:       runsList,
		prNumber:       "34285",
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
	case initMsg:
		m.checks = msg.checks
		m.runs = msg.runs
		checkItems := make([]list.Item, 0)
		for _, check := range m.checks {
			it := item{title: check.Name, description: check.Workflow}
			checkItems = append(checkItems, it)
		}
		runItems := make([]list.Item, 0)
		for _, run := range m.runs {
			it := item{title: run.Name, description: run.Link}
			runItems = append(runItems, it)
		}
		cmd = m.checksList.SetItems(checkItems)
		cmds = append(cmds, cmd)
		cmd = m.runsList.SetItems(runItems)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.checksList.SetHeight(msg.Height)
		m.runsList.SetHeight(msg.Height)
		m.logsViewport.SetHeight(msg.Height - 1)
		m.logsViewport.Style.Width(m.logsViewport.Width())
		m.logsViewport.SetContent("Ipsum excepteur voluptate ipsum excepteur.\nAdipisicing reprehenderit proident exercitation nostrud nostrud commodo exercitation aute reprehenderit adipisicing eu minim non elit sit. Lorem veniam consectetur qui Lorem consectetur quis amet magna aliquip magna excepteur eu ea ad. Aliqua proident anim consectetur reprehenderit et elit officia est et.")
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
		m.runsList, cmd = m.runsList.Update(msg)
		cmds = append(cmds, cmd)
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
				lipgloss.JoinVertical(
					lipgloss.Left,
					fmt.Sprintf("model: %dx%d, vp: %dx%d", m.width, m.height, m.logsViewport.Width, m.logsViewport.Height),
					m.logsViewport.View(),
				),
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

	focusedList.Styles.Title = focusedPaneTitleStyle.PaddingLeft(0)
	focusedList.Styles.TitleBar = focusedPaneTitleStyle.MarginBottom(1)
	focusedDelegate.Styles.SelectedTitle = focusedPaneItemTitleStyle
	focusedDelegate.Styles.SelectedDesc = focusedPaneItemDescStyle
	focusedDelegate.Styles.NormalDesc = normalItemDescStyle
	focusedList.SetDelegate(focusedDelegate)

	unfocusedList.Styles.Title = unfocusedPaneTitleStyle.PaddingLeft(0)
	unfocusedList.Styles.TitleBar = unfocusedPaneTitleStyle.MarginBottom(1)
	unfocusedDelegate.Styles.SelectedTitle = unfocusedPaneItemTitleStyle
	unfocusedDelegate.Styles.SelectedDesc = unfocusedPaneItemDescStyle
	unfocusedDelegate.Styles.NormalDesc = normalItemDescStyle
	unfocusedList.SetDelegate(unfocusedDelegate)

	m.runsList.Styles.Title = m.runsList.Styles.Title.Width(firstPaneWidth)
	m.runsList.Styles.TitleBar = m.runsList.Styles.TitleBar.Width(firstPaneWidth + 1)
	m.checksList.Styles.Title = m.checksList.Styles.Title.Width(secondPaneWidth)
	m.checksList.Styles.TitleBar = m.checksList.Styles.TitleBar.Width(secondPaneWidth + 1)
}
