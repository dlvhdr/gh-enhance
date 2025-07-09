package ui

import (
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

type workflowRunsFetchedMsg struct {
	err  error
	runs []WorkflowRun
}

func (m model) makeGetPRChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		checkRunsRes, err := api.FetchPRCheckRuns(m.repo, prNumber)
		if err != nil {
			log.Error("error fetching pr checks", "err", err)
			return workflowRunsFetchedMsg{err: err}
		}

		checkNodes := checkRunsRes.Resource.PullRequest.StatusCheckRollup.Contexts.Nodes
		checkRuns := make([]api.CheckRun, 0)
		for _, node := range checkNodes {
			checkRuns = append(checkRuns, node.CheckRun)
		}

		log.Debug("fetched pr checks", "repo", m.repo, "prNumber", prNumber, "len(checks)", len(checkRuns))
		runsMap := make(map[string]WorkflowRun)

		for _, statusCheck := range checkRuns {
			wf := statusCheck.CheckSuite.WorkflowRun.Workflow
			name := wf.Name
			if name == "" {
				name = statusCheck.Name
			}

			kind := JobKindGithubActions
			if statusCheck.CheckSuite.WorkflowRun.Workflow.Name == "GitHub Actions" {
				kind = JobKindGithubActions
			}
			runLink := statusCheck.CheckSuite.WorkflowRun.Url
			jobId := statusCheck.DatabaseId

			job := WorkflowJob{
				Id:          fmt.Sprintf("%d", jobId),
				State:       api.Conclusion(statusCheck.Status),
				Name:        name,
				Workflow:    wf.Name,
				Event:       "",
				Logs:        []LogsWithTime{},
				Loading:     false,
				Link:        runLink,
				Steps:       []api.Step{},
				StartedAt:   statusCheck.StartedAt,
				CompletedAt: statusCheck.CompletedAt,
				Bucket:      getConclusionBucket(statusCheck.Conclusion),
				Kind:        kind,
			}

			run, ok := runsMap[name]
			if ok {
				run.Jobs = append(run.Jobs, job)
			} else {
				run = WorkflowRun{
					Id:       fmt.Sprintf("%d", statusCheck.CheckSuite.WorkflowRun.DatabaseId),
					Name:     statusCheck.Name,
					Link:     runLink,
					Workflow: wf.Name,
					Event:    statusCheck.CheckSuite.WorkflowRun.Event,
					Bucket:   getConclusionBucket(statusCheck.Conclusion),
				}
				run.Jobs = []WorkflowJob{job}
			}
			runsMap[name] = run
		}

		runs := make([]WorkflowRun, 0)
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

			if runs[i].Bucket == CheckBucketFail {
				return true
			}

			if runs[j].Bucket == CheckBucketFail {
				return false
			}

			return strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)) == -1
		})

		return workflowRunsFetchedMsg{
			runs: runs,
		}
	}
}

type jobLogsFetchedMsg struct {
	jobId string
	logs  []LogsWithTime
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
		return nil
	}

	ri := m.runsList.SelectedItem().(*runItem)
	if len(ri.jobsItems) == 0 {
		return nil
	}

	job := ri.jobsItems[m.jobsList.Cursor()]
	job.initiatedLogsFetch = true
	return func() tea.Msg {
		if job.job.Kind == JobKindCheckRun || job.job.Kind == JobKindExternal {
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

		// Kind is JobKindGithubActions
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

type workflowRunStepsFetchedMsg struct {
	runId string
	data  api.WorkflowRunStepsQuery
}

func (m *model) makeFetchWorkflowRunStepsCmd(runId string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching all workflow run steps", "repo", m.repo, "runId", runId)
		jobsWithStepsRes, err := api.FetchWorkflowRunSteps(m.repo, runId)
		if err != nil {
			log.Error("error fetching all workflow run steps", "repo", m.repo,
				"prNumber", m.prNumber, "runId", runId, "err", err)
			return nil
		}

		return workflowRunStepsFetchedMsg{
			runId: runId,
			data:  jobsWithStepsRes,
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
