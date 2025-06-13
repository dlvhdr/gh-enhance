package ui

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/v2"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type runsFetchedMsg struct {
	err    error
	runs   []api.Run
	checks []api.Check
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		checksOutput, stderr, err := gh.Exec("pr", "checks", prNumber, "-R", m.repo, "--json", "name,workflow,link")
		if err != nil {
			log.Error("error fetching pr checks", "err", err, "stderr", stderr.String())
			return runsFetchedMsg{err: err}
		}

		checks := []api.Check{}

		if err := json.Unmarshal(checksOutput.Bytes(), &checks); err != nil {
			log.Error("error parsing checkouts json", "err", err)
			return runsFetchedMsg{err: err}
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
				runs = append(runs, api.Run{Name: name, Link: check.Link, Workflow: check.Workflow})
			}
		}

		return runsFetchedMsg{
			runs:   runs,
			checks: checks,
		}
	}
}

type jobLogsFetchedMsg struct {
	err   error
	jobId string
	logs  string
}

func (m *model) makeFetchJobLogsCmd() tea.Cmd {
	jobId := m.checksList.SelectedItem().(checkItem).id
	for _, job := range m.checks {
		if job.id == jobId && job.loading == false {
			log.Debug("using cached job logs", "jobId", jobId)
			m.logsViewport.SetContent(job.logs)
			return nil
		}
	}

	return func() tea.Msg {
		log.Debug("fetching logs for job", "jobId", jobId)
		jobOutput, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", jobId)
		if err != nil {
			log.Error("error fetching job logs", "jobId", jobId, "err", err, "stderr", stderr.String())
			return jobLogsFetchedMsg{err: err}
		}

		log.Debug("success fetching job logs", "jobId", jobId)
		return jobLogsFetchedMsg{
			jobId: jobId,
			logs:  jobOutput.String(),
		}
	}
}
