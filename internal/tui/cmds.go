package tui

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
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

func (m model) makeInitialGetPRChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		return m.fetchPRChecksWithCursor(prNumber, "")
	}
}

func (m model) makeGetNextPagePRChecksCmd(endCursor string) tea.Cmd {
	return func() tea.Msg {
		return m.fetchPRChecksWithCursor(m.prNumber, endCursor)
	}
}

func (m model) fetchPRChecksWithInterval() tea.Cmd {
	return tea.Tick(time.Second*10, func(t time.Time) tea.Msg {
		return m.fetchPRChecks(m.prNumber)
	})
}

func (m *model) fetchPRChecks(prNumber string) tea.Msg {
	return m.fetchPRChecksWithCursor(prNumber, "")
}

func (m model) fetchPRChecksWithCursor(prNumber string, cursor string) tea.Msg {
	response, err := api.FetchPRCheckRuns(m.repo, prNumber, cursor)
	if err != nil {
		log.Error("error fetching pr checks", "err", err)
		return workflowRunsFetchedMsg{err: err}
	}

	if response.Resource.PullRequest.Number == 0 {
		return workflowRunsFetchedMsg{err: errors.New("pull request not found")}
	}

	nodes := response.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
	runs := makeWorkflowRuns(nodes)

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
	return tea.Batch(m.runsList.StartSpinner(), m.logsSpinner.Tick, m.jobsList.StartSpinner(),
		m.makeInitialGetPRChecksCmd(m.prNumber))
}

func workflowName(cr api.CheckRun) string {
	wfName := ""
	wfr := cr.CheckSuite.WorkflowRun
	isGHA := cr.CheckSuite.App.Name == api.GithubActionsAppName
	if !isGHA {
		wfName = cr.CheckSuite.App.Name
	} else {
		wfName = wfr.Workflow.Name
	}
	if wfName == "" {
		wfName = cr.Name
	}
	return wfName
}

func jobKind(cr api.CheckRun) data.JobKind {
	isGHA := cr.CheckSuite.App.Name == api.GithubActionsAppName
	var kind data.JobKind
	if isGHA {
		kind = data.JobKindGithubActions
	} else if !strings.HasPrefix(cr.DetailsUrl, "https://github.com/") {
		kind = data.JobKindExternal
	} else {
		kind = data.JobKindCheckRun
	}

	return kind
}

func (m *model) mergeWorkflowRuns(msg workflowRunsFetchedMsg) {
	runsMap := make(map[string]data.WorkflowRun)

	// start with existing workflow runs to keep order and
	// prevent the UI from jumping
	for _, run := range m.workflowRuns {
		runsMap[run.Name] = run
	}

	for _, run := range msg.runs {
		existing, ok := runsMap[run.Name]
		log.Debug("merging runs", "run", run.Name)

		// run is new, no need to merge its jobs with the existing one
		if !ok {
			runsMap[run.Name] = run
			log.Debug("no need to merge", "run", run.Name)
			continue
		}

		// run already exists, merge its jobs with the existing one
		existing.Jobs = append(existing.Jobs, run.Jobs...)
		log.Debug("merging", "run", run.Name)
		runsMap[run.Name] = existing
	}

	runs := make([]data.WorkflowRun, 0)
	for _, run := range runsMap {
		latestJobs := takeOnlyLatestRunAttempts(run.Jobs)
		run.Jobs = latestJobs
		run.SortJobs()
		runs = append(runs, run)
	}

	m.workflowRuns = runs
}

// Create workflow runs and their jobs under data the tui can work with
// E.g. aggregate the check runs (i.e jobs) under workflow runs,
// sort jobs by their status and creation time etc.
func makeWorkflowRuns(nodes []api.ContextNode) []data.WorkflowRun {
	checkRuns := filterForCheckRuns(nodes)
	runsMap := make(map[string]data.WorkflowRun)

	for _, checkRun := range checkRuns {
		job := makeWorkflowJob(checkRun)

		wfName := workflowName(checkRun)
		run, ok := runsMap[wfName]
		if ok {
			run.Jobs = append(run.Jobs, job)
		} else {
			run = makeWorkflowRun(checkRun)
			run.Jobs = []data.WorkflowJob{job}
		}

		runsMap[wfName] = run
	}

	runs := make([]data.WorkflowRun, 0)
	for _, run := range runsMap {
		latestJobs := takeOnlyLatestRunAttempts(run.Jobs)
		run.Jobs = latestJobs
		run.SortJobs()
		runs = append(runs, run)
	}

	return runs
}

func makeWorkflowRun(checkRun api.CheckRun) data.WorkflowRun {
	wfName := workflowName(checkRun)
	link := checkRun.CheckSuite.WorkflowRun.Url
	if link == "" {
		link = checkRun.Url
	}
	var id int
	if checkRun.CheckSuite.WorkflowRun.DatabaseId == 0 {
		id = checkRun.CheckSuite.DatabaseId
	} else {
		id = checkRun.CheckSuite.WorkflowRun.DatabaseId
	}

	if id == 0 {
		log.Error("run has no ID", "workflowRun", checkRun.CheckSuite.WorkflowRun, "checkRun", checkRun)
	}

	run := data.WorkflowRun{
		Id:       fmt.Sprintf("%d", id),
		Name:     wfName,
		Link:     link,
		Workflow: checkRun.CheckSuite.WorkflowRun.Workflow.Name,
		Event:    checkRun.CheckSuite.WorkflowRun.Event,
		Bucket:   data.GetConclusionBucket(checkRun.CheckSuite.Conclusion),
	}
	return run
}

func makeWorkflowJob(checkRun api.CheckRun) data.WorkflowJob {
	pendingEnv := ""
	wfr := checkRun.CheckSuite.WorkflowRun
	if len(wfr.PendingDeploymentRequests.Nodes) > 0 {
		pendingEnv = wfr.PendingDeploymentRequests.Nodes[0].Environment.Name
	}

	kind := jobKind(checkRun)
	job := data.WorkflowJob{
		Id:          fmt.Sprintf("%d", checkRun.DatabaseId),
		Title:       checkRun.Title,
		State:       checkRun.Status,
		Conclusion:  checkRun.Conclusion,
		Name:        checkRun.Name,
		Workflow:    wfr.Workflow.Name,
		PendingEnv:  pendingEnv,
		Event:       wfr.Event,
		Logs:        []data.LogsWithTime{},
		Link:        checkRun.Url,
		Steps:       []api.Step{},
		StartedAt:   checkRun.StartedAt,
		CompletedAt: checkRun.CompletedAt,
		Bucket:      data.GetConclusionBucket(checkRun.Conclusion),
		Kind:        kind,
		RunNumber:   wfr.RunNumber,
	}
	return job
}

// Clean duplicate check runs because of old attempts.
func takeOnlyLatestRunAttempts(jobs []data.WorkflowJob) []data.WorkflowJob {
	type latestMap struct {
		runs   []data.WorkflowJob
		number int
	}
	latestRuns := map[string]latestMap{}
	for _, job := range jobs {
		wfName := job.Workflow
		existing, ok := latestRuns[wfName]
		if !ok || existing.number <
			job.RunNumber {
			r := make([]data.WorkflowJob, 0)
			r = append(r, job)
			latestRuns[wfName] = latestMap{
				runs:   r,
				number: job.RunNumber,
			}
		} else if ok {
			existing.runs = append(existing.runs, job)
			latestRuns[wfName] = latestMap{runs: existing.runs, number: existing.number}
		}
	}

	flat := make([]data.WorkflowJob, 0)
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

	commits := m.pr.Commits.Nodes
	if len(commits) > 0 {
		commits[0].Commit.StatusCheckRollup.State = api.CommitStatePending
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

type reRunRunMsg struct {
	runId string
	err   error
}

func (m *model) rerunRun(runId string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getRunItemById(runId)
	if ri == nil {
		return cmds
	}

	commits := m.pr.Commits.Nodes
	if len(commits) > 0 {
		commits[0].Commit.StatusCheckRollup.State = api.CommitStatePending
	}
	ri.run.Event = "manual rerun"
	ri.run.Bucket = data.CheckBucketPending
	ri.run.Jobs = make([]data.WorkflowJob, 0)
	ri.jobsItems = make([]*jobItem, 0)
	m.jobsList.SetItems(make([]list.Item, 0))
	m.stepsList.SetItems(make([]list.Item, 0))

	cmds = append(cmds, ri.Tick(), func() tea.Msg {
		return reRunRunMsg{runId: runId, err: api.ReRunRun(m.repo, runId)}
	})
	return cmds
}

func filterForCheckRuns(nodes []api.ContextNode) []api.CheckRun {
	checkRuns := make([]api.CheckRun, 0)
	for _, node := range nodes {
		if node.Typename != "CheckRun" {
			continue
		}
		checkRuns = append(checkRuns, node.CheckRun)
	}
	return checkRuns
}
