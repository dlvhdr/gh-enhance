package ui

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/v2"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type initMsg struct {
	err    error
	runs   []api.Run
	checks []api.Check
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		checksOutput, stderr, err := gh.Exec("pr", "checks", prNumber, "-R", m.repo, "--json", "name,workflow,link")
		if err != nil {
			log.Error("error fetching pr checks", "err", err, "stderr", stderr.String())
			return initMsg{err: err}
		}

		checks := []api.Check{}

		if err := json.Unmarshal(checksOutput.Bytes(), &checks); err != nil {
			log.Error("error parsing checkouts json", "err", err)
			return initMsg{err: err}
		}
		log.Debug("fetched pr checks", "len(checks)", len(checks))
		exist := make(map[string]bool)

		runs := make([]api.Run, 0)
		for _, check := range checks {
			name := check.Workflow
			if name == "" {
				name = check.Name
			}
			if _, ok := exist[name]; !ok {
				exist[name] = true
				runs = append(runs, api.Run{Name: name, Link: check.Link})
			}
		}

		return initMsg{
			runs:   runs,
			checks: checks,
		}
	}
}
