package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type errMsg error

type model struct {
	runId    string
	repo     string
	run      api.Run
	list     list.Model
	spinner  spinner.Model
	quitting bool
	err      error
}

func NewModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	jobsList := list.New([]list.Item{}, newItemDelegate(), 0, 0)
	jobsList.Title = "Jobs"
	jobsList.SetSize(80, 40)
	return model{list: jobsList, runId: "15372877842", repo: "port-labs/port", spinner: s}
}

func (m model) Init() tea.Cmd {
	return m.makeGetJobsCmd(m.runId)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case initMsg:

		m.run = msg.run
		jobs := make([]list.Item, 0)
		for _, job := range m.run.Jobs {
			it := item{title: job.Name, description: strings.ToTitle(job.Conclusion)}
			jobs = append(jobs, it)
		}
		cmd = m.list.SetItems(jobs)

	case tea.KeyMsg:
		if key.Matches(msg, quitKeys) {
			m.quitting = true
			return m, tea.Quit

		}
		return m, nil
	case errMsg:
		m.err = msg
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return m.err.Error()
	}

	return m.list.View()
}
