package model

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/go-gh/v2"
)

type errMsg error

type model struct {
	runId    string
	repo     string
	run      run
	list     list.Model
	spinner  spinner.Model
	quitting bool
	err      error
}

var quitKeys = key.NewBinding(
	key.WithKeys("q", "esc", "ctrl+c"),
	key.WithHelp("", "press q to quit"),
)

func newItemDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if _, ok := m.SelectedItem().(item); ok {
		} else {
			return nil
		}

		// switch msg := msg.(type) {
		// case tea.KeyPressMsg:
		// }

		return nil
	}

	help := []key.Binding{}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

func New() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	jobsList := list.New([]list.Item{}, newItemDelegate(), 0, 0)
	jobsList.Title = "Jobs"
	jobsList.SetSize(80, 40)
	return model{list: jobsList, runId: "15372877842", repo: "port-labs/port", spinner: s}
}

type initMsg struct {
	err error
	run run
}

type job struct {
	CompletedAt string
	Conclusion  string
	Name        string
	DatabaseId  int
	StartedAt   string
	Status      string
	Steps       []step
}

type step struct {
	CompletedAt string
	Conclusion  string
	Name        string
	Number      int
	StartedAt   string
	Status      string
}

type run struct {
	Jobs []job
}

func (m model) makeGetJobsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		runOutput, _, err := gh.Exec("run", "view", runId, "-R", m.repo, "--json", "jobs")
		if err != nil {
			return initMsg{err: err}
		}

		res := run{}

		if err := json.Unmarshal(runOutput.Bytes(), &res); err != nil {
			return initMsg{err: err}
		}

		return initMsg{
			run: res,
		}
	}
}

// return func() tea.Msg {
// 	logs, _, err := gh.Exec("run", "view", "--log", "--job", m.jobId, "-R", m.repo)
// 	if err != nil {
// 		return initMsg{err: err}
// 	}
// 	return initMsg{logs: logs.String()}
// }

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
