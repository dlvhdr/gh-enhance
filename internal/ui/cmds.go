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

var (
	// e.g. https://github.com/neovim/neovim/actions/runs/15852696561/job/44690047836
	jobUrlRegex = regexp.MustCompile(`^https:\/\/github\.com\/(.*)\/(.*)\/actions\/runs\/(\d+)\/job\/(\d+)$`)
	jobSubexps  = jobUrlRegex.NumSubexp()

	// e.g. https://github.com/neovim/neovim/runs/15852696561
	checkRunRegex  = regexp.MustCompile(`^https:\/\/github\.com\/(.*)\/(.*)\/runs\/(\d+)$`)
	checkRunSubexp = checkRunRegex.NumSubexp()
)

type runsFetchedMsg struct {
	err  error
	runs []api.CheckRun
}

func (m model) makeGetPrChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching check runs", "repo", m.repo, "prNumber", prNumber)
		checkRunsRes, stderr, err := gh.Exec("pr", "checks", prNumber, "-R", m.repo,
			"--json", "name,workflow,link,state,event,startedAt,completedAt,bucket")
		if err != nil {
			log.Error("error fetching pr checks", "err", err, "stderr", stderr.String())
			return runsFetchedMsg{err: err}
		}

		statusChecks := []api.Job{}

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
			kind := api.JobKindJob
			runLink := statusCheck.Link

			// TODO: clean
			if matches != nil {
				runId = string(matches[0][3])
				jobId = string(matches[0][4])
				runLink = fmt.Sprintf("https://github.com/%s/%s/actions/runs/%s",
					string(matches[0][1]), string(matches[0][2]), runId)
			} else if matches := checkRunRegex.FindAllSubmatch([]byte(statusCheck.Link), checkRunSubexp); matches != nil {
				runId = string(matches[0][3])
				jobId = runId
				runLink = fmt.Sprintf("https://github.com/%s/%s/runs/%s",
					string(matches[0][1]), string(matches[0][2]), runId)
				kind = api.JobKindCheckRun
			} else {
				log.Error("no matches when parsing status check link", "link", statusCheck.Link)
			}
			statusCheck.Id = jobId
			statusCheck.Kind = kind

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
				run.Jobs = []api.Job{statusCheck}
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
	jobId       string
	summary     string
	text        string
	description string
	title       string
}

func (m *model) makeFetchJobLogsCmd() tea.Cmd {
	if len(m.runsList.Items()) == 0 {
		log.Debug("🚨 y like dis")
		return nil
	}

	ri := m.runsList.SelectedItem().(*runItem)
	if len(ri.jobsItems) == 0 {
		log.Debug("🚨 not fetching job logs")
		return nil
	}

	job := ri.jobsItems[m.jobsList.Cursor()]
	job.initiatedLogsFetch = true
	return func() tea.Msg {
		log.Debug("🚨 fetching job logs", "link", job.job.Link)
		if job.job.Kind == api.JobKindCheckRun {
			log.Debug("🚨 fetching check run output", "link", job.job.Link)
			output, err := api.FetchCheckRunOutput(m.repo, job.job.Id)
			if err != nil {
				log.Error("error fetching check run output", "link", job.job.Link, "err", err)
				return nil
			}
			text := output.Output.Summary
			text += "\n\n"
			text += output.Output.Text
			summary, err := parseRunOutputMarkdown(
				text,
				m.logsWidth(),
			)
			if err != nil {
				log.Error("failed rendering as markdown", "link", job.job.Link, "err", err)
				summary = output.Output.Summary
			}
			return checkRunOutputFetchedMsg{
				jobId:       job.job.Id,
				title:       output.Output.Title,
				description: output.Output.Description,
				summary:     summary,
			}
		}

		jobLogsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", job.job.Id)
		if err != nil {
			log.Error("error fetching job logs", "link", job.job.Link, "err", err, "stderr", stderr.String())
			return nil
		}
		jobLogs := jobLogsRes.String()
		log.Debug("success fetching job logs", "link", job.job.Link, "bytes", len(jobLogsRes.Bytes()))

		return jobLogsFetchedMsg{
			jobId: job.job.Id,
			logs:  parseJobLogs(jobLogs),
		}
	}
}

type runJobsStepsFetchedMsg struct {
	runId         string
	jobsWithSteps api.CheckRunJobsSteps
	err           error
}

func (m *model) makeFetchRunJobsStepsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)
		jobsWithStepsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, runId, "--json", "jobs")
		if err != nil {
			log.Error("error fetching all jobs steps for run", "repo", m.repo,
				"prNumber", m.prNumber, "runId", runId, "err", err, "stderr", stderr.String())
		}

		jobsWithSteps := api.CheckRunJobsSteps{}

		if err := json.Unmarshal(jobsWithStepsRes.Bytes(), &jobsWithSteps); err != nil {
			log.Error("error parsing run jobs with steps json", "err", err, "stderr",
				stderr.String(), "response", jobsWithStepsRes.String())
		}

		log.Debug("successfully fetched all jobs steps for run", "repo", m.repo, "prNumber", m.prNumber, "runId", runId)

		for jobIdx := range jobsWithSteps.JobsSteps {
			sort.Slice(jobsWithSteps.JobsSteps[jobIdx].Steps, func(i, j int) bool {
				return jobsWithSteps.JobsSteps[jobIdx].Steps[i].Number <
					jobsWithSteps.JobsSteps[jobIdx].Steps[j].Number
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
