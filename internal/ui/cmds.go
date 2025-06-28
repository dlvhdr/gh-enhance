package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/pkg/browser"
	"github.com/cli/go-gh/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

// https://github.com/neovim/neovim/actions/runs/15852696561/job/44690047836
var jobUrlRegex = regexp.MustCompile(`^https:\/\/github\.com\/(.*)\/(.*)\/actions\/runs\/(\d+)\/job\/(\d+)$`)

var jobSubexps = jobUrlRegex.NumSubexp()

var checkRunRegex = regexp.MustCompile(`^https:\/\/github\.com\/(.*)\/(.*)\/runs\/(\d+)$`)

var checkRunSubexp = checkRunRegex.NumSubexp()

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
			log.Debug("parsing check", "name", name, "link", statusCheck.Link)

			matches := jobUrlRegex.FindAllSubmatch([]byte(statusCheck.Link), jobSubexps)
			runId, jobId := "", ""
			runLink := statusCheck.Link

			if matches != nil {
				runId = string(matches[0][3])
				jobId = string(matches[0][4])
				runLink = fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s", string(matches[0][1]), string(matches[0][2]), runId)
			} else if matches := checkRunRegex.FindAllSubmatch([]byte(statusCheck.Link), checkRunSubexp); matches != nil {
				runId = string(matches[0][3])
				jobId = runId
				runLink = fmt.Sprintf("https://github.com/%s/%s/runs/%s", string(matches[0][1]), string(matches[0][2]), runId)
				for _, match := range matches[0] {
					log.Debug("🔵", "match", string(match))
				}
			} else {
				log.Debug("🔴 no matches", "link", statusCheck.Link)
			}
			statusCheck.Id = jobId

			run, ok := runsMap[name]
			if ok {
				run.Jobs = append(run.Jobs, statusCheck)
			} else {
				run = api.CheckRun{
					Id:       runId,
					Name:     statusCheck.Name,
					Link:     runLink,
					Workflow: statusCheck.Workflow,
					Event:    statusCheck.Event,
					Bucket:   statusCheck.Bucket,
				}
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

			if runs[i].Bucket == runs[j].Bucket {
				return strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)) == -1
			}

			if runs[i].Bucket == "fail" {
				return true
			}

			if runs[j].Bucket == "fail" {
				return false
			}

			return strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)) == -1
		})

		return runsFetchedMsg{
			runs: runs,
		}
	}
}

type jobLogsFetchedMsg struct {
	jobId string
	logs  []api.StepLogsWithTime
}

type checkRunOutputFetchedMsg struct {
	jobId   string
	summary string
	title   string
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
	if job.loadingLogs == false {
		logs := strings.Builder{}
		if job.kind == "check-run" {
			m.logsViewport.SetContent(job.summary)
		} else {
			for _, log := range job.logs {
				logs.Write([]byte(log.Log))
			}
			m.logsViewport.SetContent(logs.String())
		}
		m.logsViewport.GotoTop()
		return nil
	}

	return func() tea.Msg {
		if checkRunRegex.Match([]byte(job.job.Link)) {
			log.Debug("fetching check run output", "link", job.job.Link)
			output, err := api.FetchCheckRunOutput(m.repo, job.job.Id)
			if err != nil {
				log.Error("error fetching check run output", "checkRunId", job.job, "err", err)
				return nil
			}
			summary, err := parseRunOutputMarkdown(output.Output.Summary, m.logsWidth())
			if err != nil {
				summary = output.Output.Summary
			}
			return checkRunOutputFetchedMsg{
				jobId:   job.job.Id,
				title:   output.Output.Title,
				summary: summary,
			}
		}

		jobLogsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", job.job.Id)
		if err != nil {
			log.Error("error fetching job logs", "jobId", job.job.Id, "err", err, "stderr", stderr.String(), "job", job.job)
			return nil
		}
		jobLogs := jobLogsRes.String()
		log.Debug("success fetching job logs", "jobId", job.job.Id, "bytes", len(jobLogsRes.Bytes()))

		return jobLogsFetchedMsg{
			jobId: job.job.Id,
			logs:  parseJobLogs(jobLogs),
		}
	}

}

type runJobsStepsFetchedMsg struct {
	runId         string
	jobsWithSteps api.CheckRunJobsWithSteps
	err           error
}

func (m *model) makeFetchRunJobsWithStepsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)
		jobsWithStepsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, runId, "--json", "jobs")
		if err != nil {
			log.Error("error fetching all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId, "err", err, "stderr", stderr.String())
		}

		jobsWithSteps := api.CheckRunJobsWithSteps{}

		if err := json.Unmarshal(jobsWithStepsRes.Bytes(), &jobsWithSteps); err != nil {
			log.Error("error parsing run jobs with steps json", "err", err, "stderr", stderr.String(), "response", jobsWithStepsRes.String())
		}

		log.Debug("successfully fetched all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)

		for jobIdx := range jobsWithSteps.Jobs {
			sort.Slice(jobsWithSteps.Jobs[jobIdx].Steps, func(i, j int) bool {
				return jobsWithSteps.Jobs[jobIdx].Steps[i].Number < jobsWithSteps.Jobs[jobIdx].Steps[j].Number
			})
		}

		return runJobsStepsFetchedMsg{
			runId:         runId,
			jobsWithSteps: jobsWithSteps,
		}
	}
}

func makeOpenUrlCmd(url string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("opening run url", "url", url)
		b := browser.New("", os.Stdout, os.Stdin)
		b.Browse(url)
		return nil
	}
}
