package tui

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/cli/go-gh/pkg/browser"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	"github.com/dlvhdr/gh-enhance/internal/parser"
	"github.com/dlvhdr/gh-enhance/internal/utils"
)

type prChecksFetchedMsg struct {
	pr        api.PRWithChecks
	runs      []data.WorkflowRun
	rateLimit api.RateLimit
	cursor    string
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

func (m *model) makeInitialGetRepoChecksCmd() tea.Cmd {
	return func() tea.Msg {
		return m.fetchRepoChecksWithCursor("")
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

type repoModeIntervalFetchMsg struct {
	msg tea.Msg
}

func (m *model) makeFetchRepoChecksWithInterval() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		if m.rateLimit.Remaining == 0 && time.Now().Before(m.rateLimit.ResetAt) {
			log.Warn("rate limit reached, waiting", "m.rateLimit", m.rateLimit)
			return nil
		}

		log.Info("refreshing repo checks on interval", "time", t)
		return repoModeIntervalFetchMsg{msg: m.fetchRepoChecksWithCursor("")}
	})
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
	resp, err := m.client.FetchPRCheckRuns(m.repo, prNumber, cursor)
	if err != nil {
		log.Error("error fetching pr checks", "err", err)
		return prChecksFetchedMsg{err: err, rateLimit: resp.RateLimit, cursor: cursor}
	}

	if resp.Resource.PullRequest.Number == 0 {
		return prChecksFetchedMsg{err: errors.New("pull request not found")}
	}

	nodes := resp.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
	runs := makeWorkflowRuns(nodes)

	return prChecksFetchedMsg{
		pr:        resp.Resource.PullRequest,
		runs:      runs,
		rateLimit: resp.RateLimit,
		cursor:    cursor,
	}
}

type repoModeRunsFetchedMsg struct {
	Repo string
	Runs []data.WorkflowRun
	Err  error
}

func (m model) fetchRepoChecksWithCursor(cursor string) tea.Msg {
	resp, err := m.client.FetchRepoWorkflowRuns(m.repo, cursor)
	if err != nil {
		log.Error("error fetching repo checks", "err", err)
		return repoModeRunsFetchedMsg{Err: err}
	}

	wfRuns := make([]data.WorkflowRun, 0)
	for i, run := range resp.WorkflowRuns {
		jobsResp := api.WorkflowRunJobsResponse{}
		if i == len(resp.WorkflowRuns)-1 {
			jobsResp, err = m.client.FetchWorkflowRunJobs(m.repo, strconv.Itoa(run.Id))
			if err != nil {
				log.Error(
					"error fetching workflow run jobs",
					"runId",
					run.Id,
					"runUrl",
					run.HtmlUrl,
					"err",
					err,
				)
				return runModeFetchedMsg{err: err}
			}
		}
		run.Name = fmt.Sprintf("%s #%s", run.Name, strconv.Itoa(run.RunNumber))
		convertedRun := convertRunResponseToWorkflowRun(run, jobsResp)
		wfRuns = append(wfRuns, convertedRun)
	}

	return repoModeRunsFetchedMsg{
		Repo: m.repo,
		Runs: wfRuns,
	}
}

type runJobsFetchedMsg struct {
	runId string
	jobs  []data.WorkflowJob
	err   error
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

	ji := m.getSelectedJobItem()
	if ji.isStatusInProgress() {
		return nil
	}

	log.Info("fetching job logs", "job", ji.job.Name)
	ji.loadingLogs = true
	ji.initiatedLogsFetch = true
	return func() tea.Msg {
		defer utils.TimeTrack(time.Now(), "fetching job logs")
		if ji.job.Title != "" || ji.job.Kind == data.JobKindCheckRun ||
			ji.job.Kind == data.JobKindExternal {
			log.Debug("job is not JobKindGithubActions", "job", ji.job.Kind)
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
		log.Debug("job is JobKindGithubActions", "job", ji.job.Kind)
		log.Debug(fmt.Sprintf("executing gh run view -R %s --log --job %s", m.repo, ji.job.Id))
		jobLogsRes, err := m.client.FetchJobLogs(m.repo, ji.job.Id)
		if err != nil {
			log.Error("error fetching job logs", "kind", ji.job.Kind, "link",
				ji.job.Link, "err", err, "stderr", jobLogsRes)
			return jobLogsFetchedMsg{
				jobId:  ji.job.Id,
				err:    err,
				stderr: jobLogsRes,
			}
		}
		return jobLogsFetchedMsg{
			jobId: ji.job.Id,
			logs:  parser.ParseJobLogs(jobLogsRes),
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
		stepsRes, err := m.client.FetchWorkflowRunSteps(m.repo, runId)
		if err != nil {
			log.Error("error fetching all workflow run steps", "repo", m.repo,
				"prNumber", m.prNumber, "runId", runId, "err", err)
			return nil
		}

		return workflowRunStepsFetchedMsg{
			runId: runId,
			data:  stepsRes,
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
		stepsRes, err := m.client.FetchJobSteps(m.repo, jobId)
		if err != nil {
			log.Error(
				"error fetching job steps",
				"repo",
				m.repo,
				"prNumber",
				m.prNumber,
				"jobId",
				jobId,
				"err",
				err,
			)
			return nil
		}

		log.Info("job steps fetched", "jobId", jobId, "len(steps)", len(stepsRes.Steps))

		return checkStepsFetchedMsg{
			checkId: fmt.Sprintf("%d", stepsRes.Id),
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

func (m *model) startSpinners() []tea.Cmd {
	return []tea.Cmd{
		m.checksList.StartSpinner(),
		m.runsList.StartSpinner(),
		m.logsSpinner.Tick,
		m.jobsList.StartSpinner(),
		cachedSpinner.Tick,
	}
}

func (m *model) makeInitPRCmd() tea.Cmd {
	cmds := m.startSpinners()
	cmds = append(cmds,
		m.makeFetchPRCmd(),
		m.makeInitialGetPRChecksCmd(m.prNumber),
		m.startFetchingPRChecksWithInterval(),
	)
	return tea.Batch(cmds...)
}

func (m *model) makeInitRepoModeCmd() tea.Cmd {
	cmds := m.startSpinners()
	cmds = append(cmds,
		m.makeInitialGetRepoChecksCmd(),
		m.makeFetchRepoChecksWithInterval(),
	)
	return tea.Batch(cmds...)
}

// Run mode: fetch the workflow run and its jobs directly via REST API.
func (m *model) makeInitRunModeCmd() tea.Cmd {
	cmds := m.startSpinners()
	cmds = append(cmds,
		m.makeFetchRunCmd(),
		m.startFetchingRunWithInterval(),
	)
	return tea.Batch(cmds...)
}

type runModeFetchedMsg struct {
	runs []data.WorkflowRun
	err  error
}

func (m *model) makeFetchRunCmd() tea.Cmd {
	return func() tea.Msg {
		return m.fetchRun()
	}
}

func (m *model) fetchRun() tea.Msg {
	runResp, err := m.client.FetchWorkflowRunByID(m.repo, m.runID)
	if err != nil {
		log.Error("error fetching workflow run", "err", err)
		return runModeFetchedMsg{err: err}
	}

	jobsResp, err := m.client.FetchWorkflowRunJobs(m.repo, m.runID)
	if err != nil {
		log.Error("error fetching workflow run jobs", "err", err)
		return runModeFetchedMsg{err: err}
	}

	run := convertRunResponseToWorkflowRun(runResp, jobsResp)
	return runModeFetchedMsg{runs: []data.WorkflowRun{run}}
}

func (m *model) startFetchingRunWithInterval() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return startRunIntervalFetching{}
	})
}

type startRunIntervalFetching struct{}

func (m *model) makeFetchRunIntervalTickCmd() tea.Cmd {
	return func() tea.Msg {
		if !m.isRunModeInProgress() {
			log.Info("run has concluded - not refetching anymore")
			return nil
		}

		if m.rateLimit.Remaining == 0 && time.Now().Before(m.rateLimit.ResetAt) {
			log.Warn("rate limit reached, waiting", "m.rateLimit", m.rateLimit)
			return nil
		}

		return runModeIntervalTickMsg{msg: m.fetchRun()}
	}
}

func (m *model) fetchRunWithInterval() tea.Cmd {
	return tea.Batch(
		m.makeFetchRunCmd(),
		tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
			cmd := m.makeFetchRunIntervalTickCmd()
			return cmd()
		}),
	)
}

type runModeIntervalTickMsg struct {
	msg tea.Msg
}

type jobRelatedRun struct {
	name    string
	event   string
	attempt int
	runId   string
}

func convertJobResponseToWorkflowJob(
	j api.WorkflowRunJob,
	relatedRun jobRelatedRun,
) data.WorkflowJob {
	conclusion := api.Conclusion(strings.ToUpper(j.Conclusion))
	status := api.Status(strings.ToUpper(j.Status))

	steps := make([]api.Step, 0, len(j.Steps))
	for _, s := range j.Steps {
		steps = append(steps, api.Step{
			Conclusion:  api.Conclusion(strings.ToUpper(s.Conclusion)),
			Name:        s.Name,
			Number:      s.Number,
			StartedAt:   s.StartedAt,
			CompletedAt: s.CompletedAt,
			Status:      api.Status(strings.ToUpper(s.Status)),
		})
	}

	return data.WorkflowJob{
		Id:            fmt.Sprintf("%d", j.Id),
		State:         status,
		Conclusion:    conclusion,
		Name:          j.Name,
		WorkflowName:  relatedRun.name,
		WorkflowRunId: relatedRun.runId,
		Event:         relatedRun.event,
		Logs:          []data.LogsWithTime{},
		Link:          j.HtmlUrl,
		Steps:         steps,
		StartedAt:     j.StartedAt,
		CompletedAt:   j.CompletedAt,
		Bucket:        data.GetConclusionBucket(conclusion),
		Kind:          data.JobKindGithubActions,
		RunAttempt:    relatedRun.attempt,
		CheckSuiteId:  relatedRun.runId,
	}
}

func convertRunResponseToWorkflowRun(
	run api.WorkflowRunResponse,
	jobsResp api.WorkflowRunJobsResponse,
) data.WorkflowRun {
	jobs := make([]data.WorkflowJob, len(jobsResp.Jobs))
	for i, j := range jobsResp.Jobs {
		jobs[i] = convertJobResponseToWorkflowJob(j, jobRelatedRun{
			name:    run.Name,
			event:   run.Event,
			attempt: run.RunNumber,
		})
	}
	data.SortJobs(jobs)

	runConclusion := api.Conclusion(strings.ToUpper(run.Conclusion))

	var prNumber int
	if len(run.PullRequests) > 0 {
		prNumber = run.PullRequests[0].Number
	}

	wfRun := data.WorkflowRun{
		Id:           fmt.Sprintf("%d", run.Id),
		CheckSuiteId: run.CheckSuiteId,
		HeadSha:      run.HeadSha,
		Name:         run.Name,
		DisplayTitle: run.DisplayTitle,
		Link:         run.HtmlUrl,
		Workflow:     run.Name,
		Event:        run.Event,
		Jobs:         jobs,
		Bucket:       data.GetConclusionBucket(runConclusion),
		StartedAt:    run.RunStartedAt,
		RunNumber:    run.RunNumber,
		PRNumber:     prNumber,
		RunAttempt:   run.RunAttempt,
		Status:       run.Status,
		Conclusion:   run.Conclusion,
	}

	return wfRun
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

func (m *model) mergeFetchedWorkflowRuns(
	msg prChecksFetchedMsg,
	runsSoFar []data.WorkflowRun,
) []data.WorkflowRun {
	runsMap := make(map[string]data.WorkflowRun)

	// start with existing workflow runs to keep order and
	// prevent the UI from jumping
	for _, run := range runsSoFar {
		runsMap[run.Id] = run
	}

	for _, run := range msg.runs {
		existing, ok := runsMap[run.Id]

		// run is new, no need to merge its jobs with the existing one
		if !ok {
			log.Debug("new run")
			runsMap[run.Id] = run
			continue
		}

		// run already exists, merge its jobs with the existing one
		existing.Jobs = takeNewestJobs(existing.Jobs, run.Jobs)
		runsMap[run.Id] = existing
	}

	merged := make([]data.WorkflowRun, 0)
	for _, run := range runsMap {
		merged = append(merged, run)
	}

	data.SortRuns(merged)
	return merged
}

// Create workflow runs and their jobs under data the tui can work with
// E.g. aggregate the check runs (i.e jobs) under workflow runs (a collection of jobs),
// sort jobs by their status and creation time etc.
func makeWorkflowRuns(nodes []api.ContextNode) []data.WorkflowRun {
	checkRuns := filterForCheckRuns(nodes)
	runsMap := make(map[string]data.WorkflowRun)

	for _, checkRun := range checkRuns {
		job := makeWorkflowJob(checkRun)
		run, ok := runsMap[job.Id]
		if ok {
			run.Jobs = append(run.Jobs, job)
		} else {
			run = extractWorkflowRun(checkRun)
			run.Jobs = []data.WorkflowJob{job}
		}

		runsMap[job.Id] = run
	}

	runs := make([]data.WorkflowRun, 0)
	for _, run := range runsMap {
		runs = append(runs, run)
	}

	return runs
}

func extractWorkflowRun(checkRun api.CheckRun) data.WorkflowRun {
	wfName := workflowName(checkRun)
	link := checkRun.CheckSuite.WorkflowRun.Url
	if link == "" {
		link = checkRun.Url
	}

	var id string
	if checkRun.CheckSuite.WorkflowRun.DatabaseId != 0 {
		id = fmt.Sprintf("%d", checkRun.CheckSuite.WorkflowRun.DatabaseId)
	} else {
		id = fmt.Sprintf("%d", checkRun.CheckSuite.DatabaseId)
	}

	if id == "" {
		log.Error(
			"run has no ID",
			"workflowRun",
			checkRun.CheckSuite.WorkflowRun,
			"checkRun",
			checkRun,
		)
	}

	// This is a bit hacky, since a check run belongs to a check suite and not necessarily to a workflow run.
	// They can be associated though.
	// A workflow run is a run of a GitHub Actions workflow file, while a Check Suite is a grouping of check runs for a commit.
	run := data.WorkflowRun{
		Id:           id,
		Name:         wfName,
		CheckSuiteId: fmt.Sprintf("%d", checkRun.CheckSuite.DatabaseId),
		HeadSha:      checkRun.CheckSuite.Commit.Oid,
		DisplayTitle: wfName,
		Link:         link,
		Workflow:     checkRun.CheckSuite.WorkflowRun.Workflow.Name,
		Event:        checkRun.CheckSuite.WorkflowRun.Event,
		Bucket:       data.GetConclusionBucket(checkRun.CheckSuite.Conclusion),
		StartedAt:    checkRun.StartedAt,
		RunNumber:    checkRun.CheckSuite.WorkflowRun.RunNumber,  // might be 0 if not associated with a workflow run
		RunAttempt:   checkRun.CheckSuite.WorkflowRun.RunAttempt, // might be 0 if not associated with a workflow run
		Status:       strings.ToLower(string(checkRun.CheckSuite.Status)),
		Conclusion:   strings.ToLower(string(checkRun.Conclusion)),
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
		Id:            fmt.Sprintf("%d", checkRun.DatabaseId),
		Title:         checkRun.Title,
		State:         checkRun.Status,
		Conclusion:    checkRun.Conclusion,
		Name:          checkRun.Name,
		WorkflowName:  wfr.Workflow.Name,
		WorkflowRunId: fmt.Sprintf("%d", wfr.DatabaseId),
		PendingEnv:    pendingEnv,
		Event:         wfr.Event,
		Logs:          []data.LogsWithTime{},
		Link:          checkRun.Url,
		Steps:         []api.Step{},
		StartedAt:     checkRun.StartedAt,
		CompletedAt:   checkRun.CompletedAt,
		Bucket:        data.GetConclusionBucket(checkRun.Conclusion),
		Kind:          kind,
		RunAttempt:    wfr.RunAttempt,
		CheckSuiteId:  fmt.Sprintf("%d", checkRun.CheckSuite.DatabaseId),
	}
	return job
}

// Clean duplicate jobs
func takeNewestJobs(
	oldJobs []data.WorkflowJob,
	newJobs []data.WorkflowJob,
) []data.WorkflowJob {
	merged := make([]data.WorkflowJob, 0)
	oldJobsIndexes := map[string]int{}
	for idx, oldJob := range oldJobs {
		merged = append(merged, oldJob)
		oldJobsIndexes[oldJob.Id] = idx
	}

	for _, newJob := range newJobs {
		if idx, ok := oldJobsIndexes[newJob.Id]; ok {
			merged[idx] = newJob
		} else {
			merged = append(merged, newJob)
		}
	}

	return merged
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

	m.setPRInProgress()
	ji.job.Event = "manual rerun"
	ji.job.Bucket = data.CheckBucketPending
	ji.job.Conclusion = ""
	ji.job.State = api.StatusPending
	ji.job.StartedAt = time.Now()
	ji.job.CompletedAt = time.Time{}
	ji.stepsItems = make([]*stepItem, 0)
	ji.loadingSteps = true
	m.stepsList.ResetSelected()
	m.stepsList.ResetFilter()
	m.stepsList.SetItems(make([]list.Item, 0))

	if ri != nil {
		cmds = append(cmds, ri.Tick())
	}
	cmds = append(cmds, m.inProgressSpinner.Tick, func() tea.Msg {
		// don't return too fast as gh might not have even created the job yet
		time.Sleep(2 * time.Second)
		return reRunJobMsg{jobId: jobId, err: m.client.ReRunJob(m.repo, jobId)}
	})
	return cmds
}

type reRunRunMsg struct {
	oldRunId string
	err      error
}

func (m *model) rerunRun(runId string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0)
	ri := m.getRunItemById(runId)
	if ri == nil {
		return cmds
	}

	m.setPRInProgress()
	ri.run.Event = "manual rerun"
	ri.run.Bucket = data.CheckBucketPending
	ri.run.Jobs = nil
	ri.run.StartedAt = time.Now()
	ri.run.Conclusion = ""
	ri.run.Status = "pending"
	ri.jobsItems = nil
	ri.lastFetchJobs = time.Time{}
	ri.lastFetchSteps = time.Time{}
	m.jobsList.ResetFilter()
	m.stepsList.ResetFilter()
	m.jobsList.SetItems(make([]list.Item, 0))
	m.stepsList.SetItems(make([]list.Item, 0))

	cmds = append(cmds, ri.Tick(), func() tea.Msg {
		err := m.client.ReRunRun(m.repo, runId)

		// don't return too fast as gh might not have even created the run yet
		time.Sleep(2 * time.Second)
		return reRunRunMsg{
			oldRunId: ri.run.Id,
			err:      err,
		}
	})
	return cmds
}

type prFetchedMsg struct {
	pr  api.PR
	err error
}

func (m model) fetchPR() tea.Msg {
	resp, err := m.client.FetchPR(m.repo, m.prNumber)
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
		if m.flat {
			return PaneChecks
		}
		return PaneJobs
	}

	if m.flat {
		return PaneChecks
	}
	return PaneRuns
}

func (m *model) setPRInProgress() {
	if m.mode() != ModePR {
		return
	}

	if len(m.prWithChecks.Commits.Nodes) > 0 {
		m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.State = api.CommitStatePending
	}
}
