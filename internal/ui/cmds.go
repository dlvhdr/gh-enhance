package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log"
	"github.com/cli/go-gh/pkg/browser"
	"github.com/cli/go-gh/v2"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/utils"
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
			wfr := statusCheck.CheckSuite.WorkflowRun
			wfName := ""
			isGHA := statusCheck.CheckSuite.App.Name == api.GithubActionsAppName
			if !isGHA {
				wfName = statusCheck.CheckSuite.App.Name
			} else {
				wfName = wfr.Workflow.Name
			}
			if wfName == "" {
				wfName = statusCheck.Name
			}

			var kind JobKind
			if isGHA {
				kind = JobKindGithubActions
			} else if !strings.HasPrefix(statusCheck.DetailsUrl, "https://github.com/") {
				kind = JobKindExternal
			} else {
				kind = JobKindCheckRun
			}

			job := WorkflowJob{
				Id:          fmt.Sprintf("%d", statusCheck.DatabaseId),
				State:       statusCheck.Status,
				Conclusion:  statusCheck.Conclusion,
				Name:        statusCheck.Name,
				Workflow:    wfr.Workflow.Name,
				Event:       "",
				Logs:        []LogsWithTime{},
				Link:        statusCheck.Url,
				Steps:       []api.Step{},
				StartedAt:   statusCheck.StartedAt,
				CompletedAt: statusCheck.CompletedAt,
				Bucket:      getConclusionBucket(statusCheck.Conclusion),
				Kind:        kind,
			}

			run, ok := runsMap[wfName]
			if ok {
				run.Jobs = append(run.Jobs, job)
			} else {
				link := statusCheck.CheckSuite.WorkflowRun.Url
				if link == "" {
					link = statusCheck.Url
				}
				run = WorkflowRun{
					Id:       fmt.Sprintf("%d", statusCheck.CheckSuite.WorkflowRun.DatabaseId),
					Name:     wfName,
					Link:     link,
					Workflow: wfr.Workflow.Name,
					Event:    statusCheck.CheckSuite.WorkflowRun.Event,
					Bucket:   getConclusionBucket(statusCheck.Conclusion),
				}
				run.Jobs = []WorkflowJob{job}
			}
			runsMap[wfName] = run
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
	jobId        string
	renderedText string
	text         string
	description  string
	title        string
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
		defer utils.TimeTrack(time.Now(), "fetching job logs")
		if job.job.Kind == JobKindCheckRun || job.job.Kind == JobKindExternal {
			output, err := api.FetchCheckRunOutput(m.repo, job.job.Id)
			if err != nil {
				log.Error("error fetching check run output", "link", job.job.Link, "err", err)
				return nil
			}
			text := "# " + output.Output.Title
			text += "\n\n"
			text += output.Output.Summary
			text += "\n\n"
			text += output.Output.Text
			renderedText, err := parseRunOutputMarkdown(
				text,
				m.logsWidth(),
			)
			if err != nil {
				log.Error("failed rendering as markdown", "link", job.job.Link, "err", err)
				renderedText = text
			}
			return checkRunOutputFetchedMsg{
				jobId:        job.job.Id,
				title:        output.Output.Title,
				description:  output.Output.Description,
				renderedText: renderedText,
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
