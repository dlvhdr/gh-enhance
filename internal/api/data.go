package api

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/log/v2"
	gh "github.com/cli/go-gh/v2/pkg/api"
	checks "github.com/dlvhdr/x/gh-checks"
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

	// Check run states
	CheckRunStateQueued         CheckRunState = "QUEUED"
	CheckRunStateCompleted      CheckRunState = "COMPLETED"
	CheckRunStateInProgress     CheckRunState = "IN_PROGRESS"
	CheckRunStateRequested      CheckRunState = "REQUESTED"
	CheckRunStateWaiting        CheckRunState = "WAITING"
	CheckRunStatePending        CheckRunState = "PENDING"
	CheckRunStateActionRequired CheckRunState = "ACTION_REQUIRED"
	CheckRunStateCancelled      CheckRunState = "CANCELLED"
	CheckRunStateFailure        CheckRunState = "FAILURE"
	CheckRunStateNeutral        CheckRunState = "NEUTRAL"
	CheckRunStateSkipped        CheckRunState = "SKIPPED"
	CheckRunStateStale          CheckRunState = "STALE"
	CheckRunStateStartupFailure CheckRunState = "STARTUP_FAILURE"
	CheckRunStateSuccess        CheckRunState = "SUCCESS"
	CheckRunStateTimedOut       CheckRunState = "TIMED_OUT"

	GithubActionsAppName = "GitHub Actions"
)

type Status string

type Conclusion string

type CheckRunState string

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
	Branch     struct {
		Name string
	}
	App struct {
		Id   string
		Name string
	}

	// A WorkflowRun has one CheckSuite and is defined by a GitHub Action's file
	WorkflowRun struct {
		Url                       string
		DatabaseId                int
		Event                     string
		RunNumber                 int
		PendingDeploymentRequests struct {
			Nodes []struct {
				Environment struct {
					Name string
				}
			}
		} `graphql:"pendingDeploymentRequests(first: 1)"`
		Workflow struct {
			Name string
		}
	}
}

// Represents an individual commit status context
// E.g. a Vercel deployment preview
type StatusContext struct {
	Context     string
	Description string
	State       Conclusion
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
	CommitStateExpected CommitState = "EXPECTED"
	CommitStateError    CommitState = "ERROR"
	CommitStateFailure  CommitState = "FAILURE"
	CommitStatePending  CommitState = "PENDING"
	CommitStateSuccess  CommitState = "SUCCESS"
)

type PageInfo struct {
	EndCursor       string
	HasNextPage     bool
	HasPreviousPage bool
}

type ContextNode struct {
	Typename      string        `graphql:"__typename"`
	CheckRun      CheckRun      `graphql:"... on CheckRun"`
	StatusContext StatusContext `graphql:"... on StatusContext"`
}

type PR struct {
	Title      string
	Number     int
	Url        string
	Repository struct {
		NameWithOwner string
	}
	Merged      bool
	IsDraft     bool
	Closed      bool
	HeadRefName string
	Commits     struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup struct {
					State    CommitState
					Contexts struct {
						CheckRunCount              int
						CheckRunCountsByState      []checks.ContextCountByState
						StatusContextCount         int
						StatusContextCountsByState []checks.ContextCountByState
						Nodes                      []ContextNode
						PageInfo                   PageInfo
					} `graphql:"contexts(first: 100, after: $cursor)"`
				}
			}
		}
	} `graphql:"commits(last: 1)"`
}

type PRCheckRunsQuery struct {
	Resource struct {
		PullRequest PR `graphql:"... on PullRequest"`
	} `graphql:"resource(url: $url)"`
}

var client *gh.GraphQLClient

func SetClient(c *gh.GraphQLClient) {
	client = c
}

func getClient() (*gh.GraphQLClient, error) {
	var err error
	if client != nil {
		return client, nil
	}
	client, err = gh.DefaultGraphQLClient()
	return client, err
}

func FetchPRCheckRuns(repo string, prNumber string, cursor string) (PRCheckRunsQuery, error) {
	var err error
	var res PRCheckRunsQuery
	c, err := getClient()
	if err != nil {
		return res, err
	}

	parsedUrl, err := url.Parse(fmt.Sprintf("https://github.com/%s/pull/%s", repo, prNumber))
	if err != nil {
		return res, err
	}
	variables := map[string]any{
		"url":    githubv4.URI{URL: parsedUrl},
		"cursor": githubv4.String(cursor),
	}

	err = c.Query("FetchCheckRuns", &res, variables)
	if err != nil {
		log.Error("error fetching check runs", "err", err)
		return res, err
	}

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
				Branch struct {
					Name string
				}
				CheckRuns struct {
					Nodes []CheckRunWithSteps
				} `graphql:"checkRuns(first: 100)"`
			}
		} `graphql:"... on WorkflowRun"`
	} `graphql:"resource(url: $url)"`
}

func FetchWorkflowRunSteps(repo string, runID string) (WorkflowRunStepsQuery, error) {
	res := WorkflowRunStepsQuery{}
	c, err := getClient()
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
	err = c.Query("FetchCheckRunSteps", &res, variables)
	if err != nil {
		log.Error("error fetching check run steps", "err", err)
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

func (pr *PR) IsStatusCheckInProgress() bool {
	return (pr.Commits.Nodes[0].Commit.StatusCheckRollup.State == "" ||
		pr.Commits.Nodes[0].Commit.StatusCheckRollup.State == "PENDING")
}

func ReRunJob(repo string, jobId string) error {
	client, err := gh.DefaultRESTClient()
	if err != nil {
		return err
	}

	body := strings.NewReader("")
	res := struct{}{}

	err = client.Post(fmt.Sprintf("repos/%s/actions/jobs/%s/rerun", repo, jobId), body, res)
	return err
}

func ReRunRun(repo string, runId string) error {
	client, err := gh.DefaultRESTClient()
	if err != nil {
		return err
	}

	body := strings.NewReader("")
	res := struct{}{}

	err = client.Post(fmt.Sprintf("repos/%s/actions/runs/%s/rerun", repo, runId), body, res)
	return err
}
