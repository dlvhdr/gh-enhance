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
	StatusQueued     Status = "queued"
	StatusCompleted  Status = "completed"
	StatusInProgress Status = "in_progress"
	StatusRequested  Status = "requested"
	StatusWaiting    Status = "waiting"
	StatusPending    Status = "pending"

	// Check run statuses
	CheckRunStatusQueued     CheckRunStatus = "QUEUED"
	CheckRunStatusCompleted  CheckRunStatus = "COMPLETED"
	CheckRunStatusInProgress CheckRunStatus = "IN_PROGRESS"
	CheckRunStatusRequested  CheckRunStatus = "REQUESTED"
	CheckRunStatusWaiting    CheckRunStatus = "WAITING"
	CheckRunStatusPending    CheckRunStatus = "PENDING"

	// Run conclusions
	StatusCheckConclusionActionRequired StatusCheckConclusion = "ACTION_REQUIRED"
	StatusCheckConclusionCancelled      StatusCheckConclusion = "CANCELLED"
	StatusCheckConclusionFailure        StatusCheckConclusion = "FAILURE"
	StatusCheckConclusionNeutral        StatusCheckConclusion = "NEUTRAL"
	StatusCheckConclusionSkipped        StatusCheckConclusion = "SKIPPED"
	StatusCheckConclusionStale          StatusCheckConclusion = "STALE"
	StatusCheckConclusionStartupFailure StatusCheckConclusion = "STARTUP_FAILURE"
	StatusCheckConclusionSuccess        StatusCheckConclusion = "SUCCESS"
	StatusCheckConclusionTimedOut       StatusCheckConclusion = "TIMED_OUT"

	ConclusionActionRequired Conclusion = "action_required"
	ConclusionCancelled      Conclusion = "cancelled"
	ConclusionFailure        Conclusion = "failure"
	ConclusionNeutral        Conclusion = "neutral"
	ConclusionSkipped        Conclusion = "skipped"
	ConclusionStale          Conclusion = "stale"
	ConclusionStartupFailure Conclusion = "startup_failure"
	ConclusionSuccess        Conclusion = "success"
	ConclusionTimedOut       Conclusion = "timed_out"

	AnnotationFailure Level = "failure"
	AnnotationWarning Level = "warning"
)

var AllStatuses = []string{
	"queued",
	"completed",
	"in_progress",
	"requested",
	"waiting",
	"pending",
	"action_required",
	"cancelled",
	"failure",
	"neutral",
	"skipped",
	"stale",
	"startup_failure",
	"success",
	"timed_out",
}

func IsFailureStatusCheckState(c StatusCheckConclusion) bool {
	switch c {
	case StatusCheckConclusionActionRequired, StatusCheckConclusionFailure,
		StatusCheckConclusionStartupFailure, StatusCheckConclusionTimedOut:
		return true
	default:
		return false
	}
}

func IsFailureConclusion(c Conclusion) bool {
	switch c {
	case ConclusionActionRequired, ConclusionFailure, ConclusionStartupFailure, ConclusionTimedOut:
		return true
	default:
		return false
	}
}

type CheckRunStatus string

type Status string

type StatusCheckConclusion string

type Conclusion string

type Level string

type Step struct {
	Conclusion  Conclusion
	Name        string
	Number      int
	StartedAt   time.Time
	CompletedAt time.Time
	Status      Status
}

type JobSteps struct {
	DatabaseId int
	Url        string
	Steps      []Step
}

type CheckRunJobsSteps struct {
	JobsSteps []JobSteps
}

type CheckRunOutput struct {
	Title       string
	Summary     string
	Text        string
	Description string
}

type CheckRun struct {
	Id          string
	Name        string
	Status      CheckRunStatus
	Title       string
	Url         string
	Conclusion  StatusCheckConclusion
	DatabaseId  int
	StartedAt   time.Time
	CompletedAt time.Time
	CheckSuite  CheckSuite
}

type CheckRunSteps struct {
	Id         string
	DatabaseId int
	Url        string
	Steps      struct {
		Nodes []struct {
			Conclusion  Conclusion
			Name        string
			Number      int
			StartedAt   time.Time
			CompletedAt time.Time
			Status      Status
		}
	} `graphql:"steps(first: 100)"`
}

type CheckSuite struct {
	App struct {
		Id   string
		Name string
	}
	WorkflowRun struct {
		DatabaseId int
		Event      string
		RunNumber  int
		Workflow   struct {
			Name string
		}
	}
}

type CheckRunsQuery struct {
	Resource struct {
		PullRequest struct {
			Title             string
			StatusCheckRollup struct {
				Contexts struct {
					Nodes []struct {
						CheckRun CheckRun `graphql:"... on CheckRun"`
					}
				} `graphql:"contexts(first: 100)"`
			}
		} `graphql:"... on PullRequest"`
	} `graphql:"resource(url: $url)"`
}

func FetchCheckRuns(repo string, prNumber string) (CheckRunsQuery, error) {
	client, err := gh.DefaultGraphQLClient()
	res := CheckRunsQuery{}
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
					Nodes []CheckRunSteps
				} `graphql:"checkRuns(first: 100)"`
			}
		} `graphql:"... on WorkflowRun"`
	} `graphql:"resource(url: $url)"`
}

func FetchCheckRunSteps(repo string, prNumber string) (WorkflowRunStepsQuery, error) {
	client, err := gh.DefaultGraphQLClient()
	res := WorkflowRunStepsQuery{}
	if err != nil {
		return res, err
	}

	runUrl, err := url.Parse(fmt.Sprintf("https://github.com/%s/actions/runs/%s", repo, prNumber))
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

func FetchCheckRunOutput(repo string, runId string) (CheckRunOutputResponse, error) {
	client, err := gh.DefaultRESTClient()
	res := CheckRunOutputResponse{}
	if err != nil {
		return res, err
	}

	err = client.Get(fmt.Sprintf("repos/%s/check-runs/%s", repo, runId), &res)
	if err != nil {
		return res, err
	}

	return res, nil
}
