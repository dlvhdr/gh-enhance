package ui

import (
	"encoding/json"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/logs_parser"
)

const (
	stepStartMarker  = "##[group]Run "
	groupStartMarker = "##[group]"
	groupEndMarker   = "##[endgroup]"
)

type runsFetchedMsg struct {
	err  error
	runs []api.CheckRun
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching check runs", "repo", m.repo, "prNumber", prNumber)
		checkRunsRes, stderr, err := gh.Exec("pr", "checks", prNumber, "-R", m.repo, "--json", "name,workflow,link,state,event,startedAt,completedAt,bucket")
		if err != nil {
			log.Error("error fetching pr checks", "err", err, "stderr", stderr.String())
			return runsFetchedMsg{err: err}
		}

		statusChecks := []api.StatusCheck{}

		if err := json.Unmarshal(checkRunsRes.Bytes(), &statusChecks); err != nil {
			log.Error("error parsing checkouts json", "err", err)
			return runsFetchedMsg{err: err}
		}
		log.Debug("fetched pr checks", "repo", m.repo, "prNumber", prNumber, "len(checks)", len(statusChecks))
		runsMap := make(map[string]api.CheckRun)

		for _, statusCheck := range statusChecks {
			name := statusCheck.Workflow
			if name == "" {
				name = statusCheck.Name
			}

			run, ok := runsMap[name]
			if ok {
				run.Jobs = append(run.Jobs, statusCheck)
			} else {
				run = api.CheckRun{Name: statusCheck.Name, Link: statusCheck.Link, Workflow: statusCheck.Workflow, Event: statusCheck.Event, Bucket: statusCheck.Bucket}
				run.Jobs = []api.StatusCheck{statusCheck}
			}
			runsMap[name] = run
		}

		runs := make([]api.CheckRun, 0)
		for _, run := range runsMap {
			runs = append(runs, run)
		}

		sort.Slice(runs, func(i, j int) bool {
			nameA := runs[i].Workflow
			if nameA == "" {
				nameA = runs[i].Name
			}

			nameB := runs[j].Workflow
			if nameB == "" {
				nameB = runs[j].Name
			}

			return strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)) == -1
		})

		return runsFetchedMsg{
			runs: runs,
		}
	}
}

type jobLogsFetchedMsg struct {
	err   error
	jobId string
	logs  []api.StepLogsWithTime
}

func (m *model) makeFetchJobLogsCmd() tea.Cmd {
	if len(m.runsList.Items()) == 0 {
		return nil
	}

	run := m.runsList.SelectedItem().(*runItem)
	if len(run.jobs) == 0 {
		return nil
	}

	job := run.jobs[m.jobsList.Cursor()]
	if job.loading == false {
		logs := strings.Builder{}
		for _, log := range job.logs {
			logs.Write([]byte(log.Log))
		}
		m.logsViewport.SetContent(logs.String())
		return nil
	}

	return func() tea.Msg {
		jobLogsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", job.id)
		if err != nil {
			log.Error("error fetching job logs", "jobId", job.id, "err", err, "stderr", stderr.String())
		}
		jobLogs := jobLogsRes.String()
		log.Debug("success fetching job logs", "jobId", job.id, "bytes", len(jobLogsRes.Bytes()))

		return jobLogsFetchedMsg{
			jobId: job.id,
			logs:  logs_parser.MarkStepsLogsByTime(job.id, jobLogs),
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

		for jobIdx := range jobsWithSteps.Jobs {
			sort.Slice(jobsWithSteps.Jobs[jobIdx].Steps, func(i, j int) bool {
				return jobsWithSteps.Jobs[jobIdx].Steps[i].Number < jobsWithSteps.Jobs[jobIdx].Steps[j].Number
			})
		}

		return runJobsStepsFetchedMsg{
			jobsWithSteps: jobsWithSteps,
		}
	}
}
