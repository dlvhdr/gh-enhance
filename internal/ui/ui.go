package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type errMsg error

type model struct {
	prNumber   string
	repo       string
	checks     []api.Check
	runs       []api.Run
	checksList list.Model
	runsList   list.Model
	spinner    spinner.Model
	quitting   bool
	err        error
}

func NewModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	checksList := list.New([]list.Item{}, newItemDelegate(), 0, 0)
	checksList.Title = "Jobs"
	checksList.SetSize(80, 40)

	runsList := list.New([]list.Item{}, newItemDelegate(), 0, 0)
	runsList.Title = "Checks"
	runsList.SetStatusBarItemName("check", "checks")
	runsList.SetSize(20, 40)
	return model{
		checksList: checksList,
		runsList:   runsList,
		prNumber:   "34285",
		repo:       "neovim/neovim",
		spinner:    s,
	}
}

func (m model) Init() tea.Cmd {
	return m.makeGetPrChecksCmd(m.prNumber)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	cmds := make([]tea.Cmd, 0)
	log.Debug("got msg", "type", fmt.Sprintf("%T", msg), "msg", fmt.Sprintf("%+v", msg))
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

	case tea.KeyMsg:
		if m.runsList.FilterState() == list.Filtering {
			break
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
	m.runsList, cmd = m.runsList.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, m.runsList.View(), m.checksList.View())
}
