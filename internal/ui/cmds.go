package ui

import (
	"encoding/json"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type runsFetchedMsg struct {
	err  error
	runs []api.CheckRun
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching check runs", "repo", m.repo, "prNumber", prNumber)
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
		log.Debug("fetched pr checks", "repo", m.repo, "prNumber", prNumber, "len(checks)", len(jobs))
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

func (m *model) makeFetchJobLogsCmd() tea.Cmd {
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
		// results := make(chan string, 2)
		// errors := make(chan error, 2)
		// wg := sync.WaitGroup{}
		//
		// wg.Add(1)
		// go func() (stdout, stderr bytes.Buffer, err error) {
		// 	defer wg.Done()
		// 	jobsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", jobId)
		// 	if err != nil {
		// 		log.Error("error fetching job logs", "jobId", jobId, "err", err, "stderr", stderr.String())
		// 	}
		// 	errors <- err
		// 	results <- jobsRes.String()
		// 	return jobsRes, stderr, err
		// }()

		// wg.Add(1)
		// go func() (stdout, stderr bytes.Buffer, err error) {
		// 	defer wg.Done()
		// 	stepsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", jobId)
		// 	if err != nil {
		// 		log.Error("error fetching job steps", "jobId", jobId, "err", err, "stderr", stderr.String())
		// 	}
		// 	errors <- err
		// 	results <- stepsRes.String()
		// 	return
		// }()
		//
		// wg.Wait()
		// close(results)
		//

		jobsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", jobId)
		if err != nil {
			log.Error("error fetching job logs", "jobId", jobId, "err", err, "stderr", stderr.String())
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

type runJobsStepsFetchedMsg struct {
	err           error
	jobsWithSteps api.CheckRunJobsWithSteps
}

func (m *model) makeFetchRunJobsWithStepsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)
		jobsWithStepsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, runId, "--json", "jobs")
		if err != nil {
			log.Error("error fetching all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId, "err", err, "stderr", stderr.String())
		}
		log.Debug("successfully fetched all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)

		jobsWithSteps := api.CheckRunJobsWithSteps{}

		if err := json.Unmarshal(jobsWithStepsRes.Bytes(), &jobsWithSteps); err != nil {
			log.Error("error parsing run jobs json", "err", err)
			return runJobsStepsFetchedMsg{err: err}
		}

		return runJobsStepsFetchedMsg{
			jobsWithSteps: jobsWithSteps,
		}
	}
}
