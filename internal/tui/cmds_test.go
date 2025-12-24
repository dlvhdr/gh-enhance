package tui

import (
	"os"
	"testing"

	"github.com/dlvhdr/gh-enhance/internal/api"
	"github.com/dlvhdr/gh-enhance/internal/data"
	graphql "github.com/hasura/go-graphql-client"
)

func TestMergingOfSameWorkflowJobs(t *testing.T) {
	d, err := os.ReadFile("./testdata/fetchPROneContext.json")
	if err != nil {
		t.Errorf("failed reading mock data file %v", err)
	}

	res := struct{ Data api.PRCheckRunsQuery }{}
	err = graphql.UnmarshalGraphQL(d, &res)
	if err != nil {
		t.Error(err)
	}

	wfr := makeWorkflowRun(res.Data.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes[0].CheckRun)
	next := []data.WorkflowRun{
		{
			Id:       wfr.Id,
			Name:     wfr.Name,
			Link:     wfr.Link,
			Workflow: wfr.Workflow,
			Event:    wfr.Event,
			Jobs: []data.WorkflowJob{
				{
					Id:         "job2",
					State:      api.StatusCompleted,
					Conclusion: api.ConclusionSuccess,
					Name:       "job2",
					Title:      "job2",
					Workflow:   wfr.Name,
					Event:      "pull_request",
					Logs:       []data.LogsWithTime{},
					Link:       "https://github.com/dlvhdr/gh-dash/actions/runs/19991547923/job/57332991075",
					Bucket:     data.CheckBucketPass,
					Kind:       data.JobKindCheckRun,
				},
			},
			Bucket: wfr.Bucket,
		},
	}

	m := NewModel("dlvhdr/gh-dash", "1", ModelOpts{})
	m.prWithChecks = res.Data.Resource.PullRequest

	runs := makeWorkflowRuns(m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes)
	msg1 := workflowRunsFetchedMsg{runs: runs, pr: m.prWithChecks}
	m.mergeWorkflowRuns(msg1)

	if len(m.workflowRuns) != 1 {
		t.Fatalf(`expected workflow runs to have length of 1, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Logf("%+v", m.workflowRuns[0].Jobs)
		t.Fatalf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}

	msg2 := workflowRunsFetchedMsg{runs: next}
	m.mergeWorkflowRuns(msg2)

	if len(m.workflowRuns) != 1 {
		t.Errorf(`expected workflow runs to have length of 1, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 2 {
		t.Errorf(`expected jobs to have length of 2, got: %d`, len(m.workflowRuns[0].Jobs))
	}
}

func TestMergingOfDifferentWorkflowJobs(t *testing.T) {
	d, err := os.ReadFile("./testdata/fetchPROneContext.json")
	if err != nil {
		t.Errorf("failed reading mock data file %v", err)
	}

	res := struct{ Data api.PRCheckRunsQuery }{}
	err = graphql.UnmarshalGraphQL(d, &res)
	if err != nil {
		t.Error(err)
	}

	wfr := makeWorkflowRun(res.Data.Resource.PullRequest.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes[0].CheckRun)

	m := NewModel("dlvhdr/gh-dash", "1", ModelOpts{})
	m.prWithChecks = res.Data.Resource.PullRequest

	runs := makeWorkflowRuns(m.prWithChecks.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes)
	msg1 := workflowRunsFetchedMsg{runs: runs, pr: m.prWithChecks}
	m.mergeWorkflowRuns(msg1)

	if len(m.workflowRuns) != 1 {
		t.Fatalf(`expected workflow runs to have length of 1, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Logf("%+v", m.workflowRuns[0].Jobs)
		t.Fatalf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}

	next := []data.WorkflowRun{
		{
			Id:       "CR_kwDOAPphoM8AAAAKdilagx",
			Name:     "some-other-id",
			Link:     "https://github.com/neovim/neovim/runs/11111111111",
			Workflow: m.workflowRuns[0].Workflow,
			Event:    m.workflowRuns[0].Event,
			Jobs: []data.WorkflowJob{
				{
					Id:         "job1",
					State:      api.StatusCompleted,
					Conclusion: api.ConclusionSuccess,
					Name:       m.workflowRuns[0].Jobs[0].Name,
					Title:      m.workflowRuns[0].Jobs[0].Name,
					Workflow:   "some-other-workflow",
					Event:      "pull_request",
					Logs:       []data.LogsWithTime{},
					Link:       "https://github.com/neovim/neovim/actions/runs/15928656163/job/44932094595",
					Bucket:     data.CheckBucketPass,
					Kind:       data.JobKindCheckRun,
				},
			},
			Bucket: wfr.Bucket,
		},
	}
	msg2 := workflowRunsFetchedMsg{runs: next}
	m.mergeWorkflowRuns(msg2)

	if len(m.workflowRuns) != 2 {
		t.Errorf(`expected workflow runs to have length of 2, got: %d`, len(m.workflowRuns))
	}

	if len(m.workflowRuns[0].Jobs) != 1 {
		t.Errorf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[0].Jobs))
	}
	if len(m.workflowRuns[1].Jobs) != 1 {
		t.Errorf(`expected jobs to have length of 1, got: %d`, len(m.workflowRuns[1].Jobs))
	}
}
