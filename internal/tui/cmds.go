package tui

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/log/v2"
	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/browser"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type workflowRunsFetchedMsg struct {
	pr   api.PR
	runs []data.WorkflowRun
	err  error
}

func (m model) makeGetPRChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		return m.fetchPRChecks(prNumber)
	}
}

func (m model) fetchPRChecks(prNumber string) tea.Msg {
	response, err := api.FetchPRCheckRuns(m.repo, prNumber)
	if err != nil {
		log.Error("error fetching pr checks", "err", err)
		return workflowRunsFetchedMsg{err: err}
	}

	if response.Resource.PullRequest.Number == 0 {
		return workflowRunsFetchedMsg{err: errors.New("pull request not found")}
	}

	checkNodes := response.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
	checkRuns := make([]api.CheckRun, 0)
	for _, node := range checkNodes {
		if node.Typename != "CheckRun" {
			continue
		}
		checkRuns = append(checkRuns, node.CheckRun)
	}

	log.Debug("fetched pr checks", "repo", m.repo, "prNumber", prNumber, "len(checks)", len(checkRuns))
	runsMap := make(map[string]data.WorkflowRun)

	latestRuns := takeOnlyLatestRun(checkRuns)
	log.Debug("removed old runs", "len(checkRuns)", len(checkRuns), "len(latestRuns)", len(latestRuns))

	for _, statusCheck := range latestRuns {
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

		var kind data.JobKind
		if isGHA {
			kind = data.JobKindGithubActions
		} else if !strings.HasPrefix(statusCheck.DetailsUrl, "https://github.com/") {
			kind = data.JobKindExternal
		} else {
			kind = data.JobKindCheckRun
		}

		pendingEnv := ""
		if len(wfr.PendingDeploymentRequests.Nodes) > 0 {
			pendingEnv = wfr.PendingDeploymentRequests.Nodes[0].Environment.Name
		}
		job := data.WorkflowJob{
			Id:          fmt.Sprintf("%d", statusCheck.DatabaseId),
			Title:       statusCheck.Title,
			State:       statusCheck.Status,
			Conclusion:  statusCheck.Conclusion,
			Name:        statusCheck.Name,
			Workflow:    wfr.Workflow.Name,
			PendingEnv:  pendingEnv,
			Event:       wfr.Event,
			Logs:        []data.LogsWithTime{},
			Link:        statusCheck.Url,
			Steps:       []api.Step{},
			StartedAt:   statusCheck.StartedAt,
			CompletedAt: statusCheck.CompletedAt,
			Bucket:      data.GetConclusionBucket(statusCheck.Conclusion),
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
			var id int
			if statusCheck.CheckSuite.WorkflowRun.DatabaseId == 0 {
				id = statusCheck.CheckSuite.DatabaseId
			} else {
				id = statusCheck.CheckSuite.WorkflowRun.DatabaseId
			}

			if id == 0 {
				log.Error("run has no ID", "workflowRun", wfr, "statusCheck", statusCheck)
			}

			run = data.WorkflowRun{
				Id:       fmt.Sprintf("%d", id),
				Name:     wfName,
				Link:     link,
				Workflow: wfr.Workflow.Name,
				Event:    statusCheck.CheckSuite.WorkflowRun.Event,
				Bucket:   data.GetConclusionBucket(statusCheck.CheckSuite.Conclusion),
			}
			run.Jobs = []data.WorkflowJob{job}
		}
		sort.Slice(run.Jobs, func(i, j int) bool {
			if run.Jobs[i].Bucket == data.CheckBucketFail &&
				run.Jobs[j].Bucket != data.CheckBucketFail {
				return true
			}
			if run.Jobs[j].Bucket == data.CheckBucketFail &&
				run.Jobs[i].Bucket != data.CheckBucketFail {
				return false
			}
			if run.Jobs[i].StartedAt.IsZero() {
				return false
			}
			if run.Jobs[j].StartedAt.IsZero() {
				return true
			}

			return run.Jobs[i].StartedAt.Before(run.Jobs[j].StartedAt)
		})
		runsMap[wfName] = run
	}

	runs := make([]data.WorkflowRun, 0)
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

		if runs[i].Bucket == data.CheckBucketFail {
			return true
		}

		if runs[j].Bucket == data.CheckBucketFail {
			return false
		}

		return strings.Compare(strings.ToLower(nameA), strings.ToLower(nameB)) == -1
	})

	return workflowRunsFetchedMsg{
		pr:   response.Resource.PullRequest,
		runs: runs,
	}
}

type jobLogsFetchedMsg struct {
	jobId  string
	logs   []data.LogsWithTime
	err    error
	stderr string
}

type checkRunOutputFetchedMsg struct {
	jobId        string
	renderedText string
	text         string
	description  string
	title        string
}

func (m *model) makeFetchJobLogsCmd() tea.Cmd {
	if len(m.runsList.VisibleItems()) == 0 {
		return nil
	}
	ri := m.runsList.SelectedItem().(*runItem)
	if len(ri.jobsItems) == 0 {
		return nil
	}
	job := m.jobsList.SelectedItem()
	if job == nil {
		return nil
	}
	ji, ok := job.(*jobItem)
	if !ok {
		return nil
	}
	if ji.isStatusInProgress() {
		return nil
	}

	log.Debug("fetching job logs", "job", ji.job.Name)
	ji.loadingLogs = true
	ji.initiatedLogsFetch = true
	return func() tea.Msg {
		defer utils.TimeTrack(time.Now(), "fetching job logs")
		if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun || ji.job.Kind == data.JobKindExternal {
			output, err := api.FetchCheckRunOutput(m.repo, ji.job.Id)
			if err != nil {
				log.Error("error fetching check run output", "link", ji.job.Link, "err", err)
				return nil
			}
			text := "# " + output.Output.Title
			text += "\n\n"
			text += output.Output.Summary
			text += "\n\n"
			text += output.Output.Text
			renderedText, err := parser.ParseRunOutputMarkdown(
				text,
				m.logsWidth(),
			)
			if err != nil {
				log.Error("failed rendering as markdown", "link", ji.job.Link, "err", err)
				renderedText = text
			}
			return checkRunOutputFetchedMsg{
				jobId:        ji.job.Id,
				title:        output.Output.Title,
				description:  output.Output.Description,
				renderedText: renderedText,
			}
		}

		// Kind is JobKindGithubActions
		jobLogsRes, stderr, err := gh.Exec("run", "view", "-R", m.repo, "--log", "--job", ji.job.Id)
		if err != nil {
			// TODO: fetch with gh api
			// if run is still in progress, gh CLI will not fetch the logs (why???)
			// e.g.
			// gh api \
			//   -H "Accept: application/vnd.github+json" \
			//   -H "X-GitHub-Api-Version: 2022-11-28" \
			//   /repos/rapidsai/cuml/actions/jobs/46882393014/logs
			log.Error("error fetching job logs", "kind", ji.job.Kind, "link",
				ji.job.Link, "err", err, "stderr", stderr.String())
			return jobLogsFetchedMsg{
				jobId:  ji.job.Id,
				err:    err,
				stderr: stderr.String(),
			}
		}
		jobLogs := jobLogsRes.String()
		log.Debug("success fetching job logs", "link", ji.job.Link, "bytes", len(jobLogsRes.Bytes()))

		return jobLogsFetchedMsg{
			jobId: ji.job.Id,
			logs:  parser.ParseJobLogs(jobLogs),
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
		log.Info("opening run url", "url", url)
		b := browser.New("", os.Stdout, os.Stdin)
		b.Browse(url)
		return nil
	}
}

func (m *model) makeInitCmd() tea.Cmd {
	return tea.Batch(m.runsList.StartSpinner(), m.logsSpinner.Tick, m.jobsList.StartSpinner(), m.makeGetPRChecksCmd(m.prNumber))
}

func takeOnlyLatestRun(checkRuns []api.CheckRun) []api.CheckRun {
	// clean duplicate check runs because of old attempts
	type latestMap struct {
		runs   []api.CheckRun
		number int
	}
	latestRuns := map[string]latestMap{}
	for _, statusCheck := range checkRuns {
		wfName := statusCheck.CheckSuite.WorkflowRun.Workflow.Name
		existing, ok := latestRuns[wfName]
		if !ok || existing.number <
			statusCheck.CheckSuite.WorkflowRun.RunNumber {
			r := make([]api.CheckRun, 0)
			r = append(r, statusCheck)
			latestRuns[wfName] = latestMap{
				runs:   r,
				number: statusCheck.CheckSuite.WorkflowRun.RunNumber,
			}
		} else if ok {
			existing.runs = append(existing.runs, statusCheck)
			latestRuns[wfName] = latestMap{runs: existing.runs, number: existing.number}
		}
	}

	flat := make([]api.CheckRun, 0)
	for _, checkRun := range latestRuns {
		flat = append(flat, checkRun.runs...)
	}
	return flat
}

type reRunJobMsg struct {
	jobId string
	err   error
}

func (m *model) rerunJob(runId string, jobId string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getRunItemById(runId)
	ji := m.getJobItemById(jobId)
	if ri == nil || ji == nil {
		return cmds
	}

	ji.job.Bucket = data.CheckBucketPending
	ji.job.State = api.StatusPending
	ji.job.StartedAt = time.Now()
	ji.job.CompletedAt = time.Time{}
	ji.steps = make([]*stepItem, 0)
	m.stepsList.ResetSelected()
	m.stepsList.SetItems(make([]list.Item, 0))

	cmds = append(cmds, ri.Tick(), ji.Tick(), m.inProgressSpinner.Tick, func() tea.Msg {
		return reRunJobMsg{jobId: jobId, err: api.ReRunJob(m.repo, jobId)}
	})
	return cmds
}

// func (m *model) rerunWorkflow() {
// }
