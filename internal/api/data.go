package api

import (
	"fmt"
	"time"

	gh "github.com/cli/go-gh/v2/pkg/api"
)

const (
	// Run statuses
	StatusQueued     Status = "queued"
	StatusCompleted  Status = "completed"
	StatusInProgress Status = "in_progress"
	StatusRequested  Status = "requested"
	StatusWaiting    Status = "waiting"
	StatusPending    Status = "pending"

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

type CheckRun struct {
	Id       string
	Name     string
	Link     string
	Workflow string
	Event    string
	Jobs     []Job
	Bucket   string // pass / skipping / fail / cancel / pending
}

type StepLogsWithTime struct {
	Log  string
	Time time.Time
}

type JobKind int

const (
	JobKindJob JobKind = iota
	JobKindCheckRun
)

type Job struct {
	Id          string
	State       StatusCheckConclusion
	Name        string
	Workflow    string
	Event       string
	Logs        []StepLogsWithTime
	Loading     bool
	Link        string
	Steps       []Step
	StartedAt   time.Time
	CompletedAt time.Time
	Bucket      string // pass / skipping / fail / cancel / pending
	Kind        JobKind
}

type CheckRunOutput struct {
	Title       string
	Summary     string
	Text        string
	Description string
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
