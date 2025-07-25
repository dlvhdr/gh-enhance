package api

import (
	"fmt"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	gh "github.com/cli/go-gh/v2/pkg/api"
	"github.com/shurcooL/githubv4"
)

const (
	// Run statuses
	StatusQueued     Status = "QUEUED"
	StatusCompleted  Status = "COMPLETED"
	StatusInProgress Status = "IN_PROGRESS"
	StatusRequested  Status = "REQUESTED"
	StatusWaiting    Status = "WAITING"
	StatusPending    Status = "PENDING"

	// Run conclusions
	ConclusionActionRequired Conclusion = "ACTION_REQUIRED"
	ConclusionCancelled      Conclusion = "CANCELLED"
	ConclusionFailure        Conclusion = "FAILURE"
	ConclusionNeutral        Conclusion = "NEUTRAL"
	ConclusionSkipped        Conclusion = "SKIPPED"
	ConclusionStale          Conclusion = "STALE"
	ConclusionStartupFailure Conclusion = "STARTUP_FAILURE"
	ConclusionSuccess        Conclusion = "SUCCESS"
	ConclusionTimedOut       Conclusion = "TIMED_OUT"

	GithubActionsAppName = "GitHub Actions"
)

type Status string

type Conclusion string

func IsFailureConclusion(c Conclusion) bool {
	switch c {
	case ConclusionActionRequired, ConclusionFailure,
		ConclusionStartupFailure, ConclusionTimedOut:
		return true
	default:
		return false
	}
}

// CheckSuite is a grouping of CheckRuns
type CheckSuite struct {
	Conclusion Conclusion
	DatabaseId int
	App        struct {
		Id   string
		Name string
	}

	// A WorkflowRun has one CheckSuite and is defined by a GitHub Actions file
	WorkflowRun struct {
		Url        string
		DatabaseId int
		Event      string
		RunNumber  int
		Workflow   struct {
			Name string
		}
	}
}

// CheckRun is a job running in CI on a specific commit. It is part of a CheckSuite.
type CheckRun struct {
	Id          string
	Name        string
	Status      Status
	Title       string
	Url         string
	DetailsUrl  string
	Conclusion  Conclusion
	DatabaseId  int
	StartedAt   time.Time
	CompletedAt time.Time
	CheckSuite  CheckSuite
}

// CheckRunWithSteps includes some basic identifying data for the check run as well as its steps
type CheckRunWithSteps struct {
	Id         string
	DatabaseId int
	Url        string
	Steps      struct {
		Nodes []Step
	} `graphql:"steps(first: 100)"`
}

type Step struct {
	Conclusion  Conclusion
	Name        string
	Number      int
	StartedAt   time.Time
	CompletedAt time.Time
	Status      Status
}

type CommitState string

const (
	CommitStatusExpected = "EXPECTED"
	CommitStatusError    = "ERROR"
	CommitStatusFailure  = "FAILURE"
	CommitStatusPending  = "PENDING"
	CommitStatusSuccess  = "SUCCESS"
)

type PR struct {
	Title      string
	Number     int
	Url        string
	Repository struct {
		NameWithOwner string
	}
	StatusCheckRollup struct {
		State    CommitState
		Contexts struct {
			CheckRunCountsByState []struct {
				Count int
				State Conclusion
			}
			Nodes []struct {
				Typename string   `graphql:"__typename"`
				CheckRun CheckRun `graphql:"... on CheckRun"`
			}
		} `graphql:"contexts(first: 100)"`
	}
}

type PRCheckRunsQuery struct {
	Resource struct {
		PullRequest PR `graphql:"... on PullRequest"`
	} `graphql:"resource(url: $url)"`
}

func FetchPRCheckRuns(repo string, prNumber string) (PRCheckRunsQuery, error) {
	client, err := gh.DefaultGraphQLClient()
	res := PRCheckRunsQuery{}
	if err != nil {
		return res, err
	}

	parsedUrl, err := url.Parse(fmt.Sprintf("https://github.com/%s/pull/%s", repo, prNumber))
	if err != nil {
		return res, err
	}
	variables := map[string]any{
		"url": githubv4.URI{URL: parsedUrl},
	}

	err = client.Query("FetchCheckRuns", &res, variables)
	if err != nil {
		return res, err
	}

	// for _, wow := range res.Resource.PullRequest.StatusCheckRollup.Contexts.Nodes {
	// 	log.Debug("wow", "node", wow.CheckRun)
	// }

	return res, nil
}

type WorkflowRunStepsQuery struct {
	Resource struct {
		WorkflowRun struct {
			Id   string
			File struct {
				Path string
			}
			CheckSuite struct {
				CheckRuns struct {
					Nodes []CheckRunWithSteps
				} `graphql:"checkRuns(first: 100)"`
			}
		} `graphql:"... on WorkflowRun"`
	} `graphql:"resource(url: $url)"`
}

func FetchWorkflowRunSteps(repo string, runID string) (WorkflowRunStepsQuery, error) {
	client, err := gh.DefaultGraphQLClient()
	res := WorkflowRunStepsQuery{}
	if err != nil {
		return res, err
	}

	runUrl, err := url.Parse(fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, runID))
	if err != nil {
		return res, err
	}
	variables := map[string]any{
		"url": githubv4.URI{URL: runUrl},
	}

	log.Debug("fetching check run steps", "url", runUrl)
	err = client.Query("FetchCheckRunSteps", &res, variables)
	if err != nil {
		return res, err
	}

	return res, nil
}

type CheckRunOutputResponse struct {
	Id     int
	Name   string
	Url    string
	Output CheckRunOutput
}

type CheckRunOutput struct {
	Title       string
	Summary     string
	Text        string
	Description string
}

func FetchCheckRunOutput(repo string, runID string) (CheckRunOutputResponse, error) {
	client, err := gh.DefaultRESTClient()
	res := CheckRunOutputResponse{}
	if err != nil {
		return res, err
	}

	err = client.Get(fmt.Sprintf("repos/%s/check-runs/%s", repo, runID), &res)
	if err != nil {
		return res, err
	}

	return res, nil
}
