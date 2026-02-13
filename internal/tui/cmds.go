package tui

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/browser"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type workflowRunsFetchedMsg struct {
	pr        api.PRWithChecks
	runs      []data.WorkflowRun
	rateLimit api.RateLimit
	err       error
}

func (m *model) makeFetchPRCmd() tea.Cmd {
	return func() tea.Msg {
		return m.fetchPR()
	}
}

func (m *model) makeInitialGetPRChecksCmd(prNumber string) tea.Cmd {
	return func() tea.Msg {
		return m.fetchPRChecksWithCursor(prNumber, "")
	}
}

func (m *model) makeGetNextPagePRChecksCmd(endCursor string) tea.Cmd {
	return func() tea.Msg {
		return m.fetchPRChecksWithCursor(m.prNumber, endCursor)
	}
}

type prChecksIntervalTickMsg struct {
	msg tea.Msg
}

var refreshInterval = time.Second * 10

func (m *model) fetchPRChecksWithInterval() tea.Cmd {
	return tea.Batch(
		m.makeFetchPRCmd(),
		func() tea.Msg {
			return m.fetchPRChecks(m.prNumber)
		},
		tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
			if !m.prWithChecks.IsStatusCheckInProgress() {
				log.Info("all tasks have concluded - not refetching anymore")
				return nil
			}

			if m.rateLimit.Remaining == 0 && time.Now().Before(m.rateLimit.ResetAt) {
				log.Warn("rate limit reached, waiting", "m.rateLimit", m.rateLimit)
				return nil
			}

			return prChecksIntervalTickMsg{msg: m.fetchPRChecks(m.prNumber)}
		}),
	)
}

type startIntervalFetching struct{}

func (m *model) startFetchingPRChecksWithInterval() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return startIntervalFetching{}
	})
}

func (m *model) fetchPRChecks(prNumber string) tea.Msg {
	log.Info("fetching pr checks from the begginging")
	return m.fetchPRChecksWithCursor(prNumber, "")
}

func (m model) fetchPRChecksWithCursor(prNumber string, cursor string) tea.Msg {
	resp, err := api.FetchPRCheckRuns(m.repo, prNumber, cursor)
	if err != nil {
		log.Error("error fetching pr checks", "err", err)
		return workflowRunsFetchedMsg{err: err, rateLimit: resp.RateLimit}
	}

	if resp.Resource.PullRequest.Number == 0 {
		return workflowRunsFetchedMsg{err: errors.New("pull request not found")}
	}

	nodes := resp.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
	runs := makeWorkflowRuns(nodes)

	return workflowRunsFetchedMsg{
		rateLimit: resp.RateLimit,
		pr:        resp.Resource.PullRequest,
		runs:      runs,
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
	if m.flat && len(m.checksList.VisibleItems()) == 0 {
		return nil
	}
	if !m.flat && len(m.runsList.VisibleItems()) == 0 {
		return nil
	}

	var ji *jobItem
	if m.flat {
		ci := m.checksList.SelectedItem().(*checkItem)
		if ci == nil {
			return nil
		}
		ji = &ci.jobItem
	} else {
		ri := m.runsList.SelectedItem().(*runItem)
		if len(ri.jobsItems) == 0 {
			return nil
		}
		job := m.jobsList.SelectedItem()
		if job == nil {
			return nil
		}
		j, ok := job.(*jobItem)
		if !ok {
			return nil
		}
		ji = j
	}

	if ji.isStatusInProgress() {
		return nil
	}

	log.Info("fetching job logs", "job", ji.job.Name)
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

type checkStepsFetchedMsg struct {
	checkId string
	steps   []api.Step
}

func (m *model) makeFetchCheckStepsCmd(jobId string) tea.Cmd {
	return func() tea.Msg {
		log.Debug("fetching check steps", "repo", m.repo, "jobId", jobId)
		stepsRes, err := api.FetchJobSteps(m.repo, jobId)
		if err != nil {
			log.Error("error fetching job steps", "repo", m.repo, "prNumber", m.prNumber, "jobId", jobId, "err", err)
			return nil
		}

		return checkStepsFetchedMsg{
			checkId: jobId,
			steps:   stepsRes.Steps,
		}
	}
}

func makeOpenUrlCmd(url string) tea.Cmd {
	return func() tea.Msg {
		log.Info("opening url", "url", url)
		b := browser.New("", os.Stdout, os.Stdin)
		b.Browse(url)
		return nil
	}
}

func (m *model) makeInitCmd() tea.Cmd {
	return tea.Batch(m.checksList.StartSpinner(), m.runsList.StartSpinner(), m.logsSpinner.Tick, m.jobsList.StartSpinner(),
		m.makeFetchPRCmd(), m.makeInitialGetPRChecksCmd(m.prNumber), m.startFetchingPRChecksWithInterval())
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
	runsMap := make(map[int]data.WorkflowRun)

	// start with existing workflow runs to keep order and
	// prevent the UI from jumping
	for _, run := range m.workflowRuns {
		runsMap[run.RunNumber] = run
	}

	for _, run := range msg.runs {
		existing, ok := runsMap[run.RunNumber]

		// run is new, no need to merge its jobs with the existing one
		if !ok {
			runsMap[run.RunNumber] = run
			continue
		}

		// run already exists, merge its jobs with the existing one
		existing.Jobs = append(existing.Jobs, run.Jobs...)
		runsMap[run.RunNumber] = existing
	}

	runs := make([]data.WorkflowRun, 0)
	for _, run := range runsMap {
		latestJobs := takeOnlyLatestRunAttempts(run.Jobs)
		run.Jobs = latestJobs
		run.SortJobs()
		runs = append(runs, run)
	}

	data.SortRuns(runs)

	m.workflowRuns = runs
}

// Create workflow runs and their jobs under data the tui can work with
// E.g. aggregate the check runs (i.e jobs) under workflow runs (a collection of jobs),
// sort jobs by their status and creation time etc.
func makeWorkflowRuns(nodes []api.ContextNode) []data.WorkflowRun {
	checkRuns := filterForCheckRuns(nodes)
	runsMap := make(map[int]data.WorkflowRun)

	for _, checkRun := range checkRuns {
		job := makeWorkflowJob(checkRun)

		wfRunNumber := checkRun.CheckSuite.WorkflowRun.RunNumber
		// wfName := workflowName(checkRun)
		run, ok := runsMap[wfRunNumber]
		if ok {
			run.Jobs = append(run.Jobs, job)
		} else {
			run = makeWorkflowRun(checkRun)
			run.Jobs = []data.WorkflowJob{job}
		}

		runsMap[wfRunNumber] = run
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
		Id:        fmt.Sprintf("%d", id),
		Name:      wfName,
		Link:      link,
		Workflow:  checkRun.CheckSuite.WorkflowRun.Workflow.Name,
		Event:     checkRun.CheckSuite.WorkflowRun.Event,
		Bucket:    data.GetConclusionBucket(checkRun.CheckSuite.Conclusion),
		StartedAt: checkRun.StartedAt,
		RunNumber: checkRun.CheckSuite.WorkflowRun.RunNumber,
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
		jobs      []data.WorkflowJob
		runNumber int
	}

	jobIds := map[string]bool{}
	wfNameToJobs := map[string]latestMap{}
	for _, job := range jobs {
		wfName := job.Workflow
		existing, ok := wfNameToJobs[wfName]

		// if the job's wf isn't yet set in the map
		if !ok {
			onlyJob := make([]data.WorkflowJob, 0)
			onlyJob = append(onlyJob, job)
			wfNameToJobs[wfName] = latestMap{
				jobs:      onlyJob,
				runNumber: job.RunNumber,
			}

			// job is part of a wf that we already met
			// and it's a later attempt of the same job
			// override the existing job with the later attempt
		} else if job.RunNumber > existing.runNumber {
			found := 0
			for i, ej := range existing.jobs {
				if ej.Name == job.Name {
					found = i
					break
				}
			}
			existing.jobs[found] = job
			wfNameToJobs[wfName] = latestMap{jobs: existing.jobs, runNumber: job.RunNumber}

			// the job isn't a later attempt - it's a job we haven't met before, append it
		} else {
			_, ok := jobIds[job.Id]
			if !ok {
				existing.jobs = append(existing.jobs, job)
			}
			wfNameToJobs[wfName] = latestMap{jobs: existing.jobs, runNumber: existing.runNumber}
		}

		jobIds[job.Id] = true
	}

	flat := make([]data.WorkflowJob, 0)
	for _, checkRun := range wfNameToJobs {
		flat = append(flat, checkRun.jobs...)
	}
	return flat
}

type reRunJobMsg struct {
	jobId string
	err   error
}

func (m *model) rerunJob(runId string, jobId string) []tea.Cmd {
	log.Info("re-running job", "runId", runId, "jobId", jobId)
	cmds := make([]tea.Cmd, 0)
	ri := m.getRunItemById(runId)
	ji := m.getJobItemById(jobId)
	if ri == nil && ji == nil {
		return cmds
	}

	commits := m.prWithChecks.Commits.Nodes
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

	if ri != nil {
		cmds = append(cmds, ri.Tick())
	}
	cmds = append(cmds, ji.Tick(), m.inProgressSpinner.Tick, func() tea.Msg {
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

	commits := m.prWithChecks.Commits.Nodes
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

type prFetchedMsg struct {
	pr  api.PR
	err error
}

func (m model) fetchPR() tea.Msg {
	resp, err := api.FetchPR(m.repo, m.prNumber)
	if err != nil {
		log.Error("error fetching pr", "err", err)
		return prFetchedMsg{err: err}
	}

	if resp.Resource.PullRequest.Number == 0 {
		return prFetchedMsg{err: errors.New("pull request not found")}
	}

	return prFetchedMsg{
		pr: resp.Resource.PullRequest,
	}
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

func (m *model) nextPane() pane {
	showSteps := m.shouldShowSteps()
	switch m.focusedPane {
	case PaneRuns:
		return PaneJobs

	case PaneJobs:
		if showSteps {
			return PaneSteps
		}

	case PaneSteps:
		return PaneLogs

	case PaneChecks:
		if showSteps {
			return PaneSteps
		}
		return PaneLogs

	case PaneLogs:
		return PaneLogs
	}

	return PaneLogs
}

func (m *model) previousPane() pane {
	showSteps := m.shouldShowSteps()
	switch m.focusedPane {
	case PaneRuns:
		return PaneRuns

	case PaneJobs:
		return PaneRuns

	case PaneSteps:
		if m.flat {
			return PaneChecks
		}
		return PaneJobs

	case PaneChecks:
		return PaneChecks

	case PaneLogs:
		if showSteps {
			return PaneSteps
		}
		return PaneJobs
	}

	if m.flat {
		return PaneChecks
	}
	return PaneRuns
}
