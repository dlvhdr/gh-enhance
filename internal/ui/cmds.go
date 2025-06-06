package ui

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cli/go-gh/v2"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type initMsg struct {
	err error
	run api.Run
}

func (m model) makeGetJobsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		runOutput, _, err := gh.Exec("run", "view", runId, "-R", m.repo, "--json", "jobs")
		if err != nil {
			return initMsg{err: err}
		}

		res := api.Run{}

		if err := json.Unmarshal(runOutput.Bytes(), &res); err != nil {
			return initMsg{err: err}
		}

		return initMsg{
			run: res,
		}
	}
}
