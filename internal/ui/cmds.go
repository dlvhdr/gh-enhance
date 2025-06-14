package ui

import (
	"encoding/json"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/v2"

	"github.com/charmbracelet/bubbletea-app-template/internal/api"
)

type runsFetchedMsg struct {
	err  error
	runs []api.CheckRun
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		checkRunsRes, stderr, err := gh.Exec("pr", "checks", prNumber, "-R", m.repo, "--json", "name,workflow,link,state")
		if err != nil {
			log.Error("error fetching pr checks", "err", err, "stderr", stderr.String())
			return runsFetchedMsg{err: err}
		}

		jobs := []api.Job{}

		if err := json.Unmarshal(checkRunsRes.Bytes(), &jobs); err != nil {
			log.Error("error parsing checkouts json", "err", err)
			return runsFetchedMsg{err: err}
		}
		log.Debug("fetched pr checks", "len(checks)", len(jobs))
		runsMap := make(map[string]api.CheckRun)

		for _, job := range jobs {
			name := job.Workflow
			if name == "" {
				name = job.Name
			}

			run, ok := runsMap[name]
			if ok {
				run.Jobs = append(run.Jobs, job)
			} else {
				run = api.CheckRun{Name: name, Link: job.Link, Workflow: job.Workflow}
				run.Jobs = []api.Job{job}
			}
			runsMap[name] = run
		}

		runs := make([]api.CheckRun, 0)
		for _, run := range runsMap {
			runs = append(runs, run)
		}

		return runsFetchedMsg{
			runs: runs,
		}
	}
}

type jobLogsFetchedMsg struct {
	err   error
	jobId string
	logs  string
}

func (m *model) makeFetchJobStepsAndLogsCmd() tea.Cmd {
	if m.jobsList.SelectedItem() == nil {
		return nil
	}
	run := m.runsList.SelectedItem().(runItem)
	jobId := m.jobsList.SelectedItem().(jobItem).id
	for _, job := range run.jobs {
		if job.id == jobId && job.loading == false {
			log.Debug("using cached job logs", "jobId", jobId)
			m.logsViewport.SetContent(job.logs)
			return nil
		}
	}

	return func() tea.Msg {
		log.Debug("fetching logs for job", "jobId", jobId)
		jobsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", jobId)
		if err != nil {
			log.Error("error fetching job logs", "jobId", jobId, "err", err, "stderr", stderr.String())
			return jobLogsFetchedMsg{err: err}
		}

		jobsStr := jobsRes.String()
		lines := strings.Lines(jobsStr)
		parsed := make([]string, 0)
		var name, step string
		count := 0
		fieldsFunc := func(r rune) bool {
			if r == '\t' {
				return true
			}
			return false
		}
		for line := range lines {
			f := strings.FieldsFunc(line, fieldsFunc)
			if len(f) < 3 {
				parsed = append(parsed, line)
				continue
			}

			if count == 0 {
				name = f[0]
				step = f[1]
			}

			dateAndLog := strings.SplitN(f[2], " ", 2)
			if len(dateAndLog) == 2 {
				d, err := time.Parse(time.RFC3339, dateAndLog[0])
				pd := strings.Repeat(" ", 8)
				if err == nil {
					pd = d.Format(time.TimeOnly)
				}

				parsed = append(parsed, strings.Join([]string{
					pd,
					lipgloss.NewStyle().Foreground(lipgloss.Color("234")).Render(""),
					dateAndLog[1],
				}, " "))
			} else {
				parsed = append(parsed, f[2])
			}
		}
		log.Debug("found fields", "count", count, "name", name, "step", step)
		if name != "" && step != "" {
			jobsStr = strings.ReplaceAll(jobsStr, name+string('\t'), "")
			jobsStr = strings.ReplaceAll(jobsStr, step+string('\t'), "")
		}

		log.Debug("success fetching job logs", "jobId", jobId)
		return jobLogsFetchedMsg{
			jobId: jobId,
			logs:  strings.Join(parsed, ""),
		}
	}
}
