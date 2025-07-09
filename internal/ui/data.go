package ui

import (
	"time"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

type WorkflowRun struct {
	Id       string
	Name     string
	Link     string
	Workflow string
	Event    string
	Jobs     []WorkflowJob
	Bucket   CheckBucket
}

type WorkflowJob struct {
	Id          string
	State       api.Conclusion
	Name        string
	Workflow    string
	Event       string
	Logs        []LogsWithTime
	Loading     bool
	Link        string
	Steps       []api.Step
	StartedAt   time.Time
	CompletedAt time.Time
	Bucket      CheckBucket
	Kind        JobKind
}

type LogsWithTime struct {
	Log  string
	Time time.Time
}

type JobKind int

const (
	JobKindJob JobKind = iota
	JobKindCheckRun
	JobKindExternal
)

type CheckBucket int

const (
	CheckBucketPass = iota
	CheckBucketSkipping
	CheckBucketFail
	CheckBucketCancel
	CheckBucketPending
)

func getConclusionBucket(conclusion api.Conclusion) CheckBucket {
	switch conclusion {
	case "SUCCESS":
		return CheckBucketPass
	case "SKIPPED", "NEUTRAL":
		return CheckBucketSkipping
	case "ERROR", "FAILURE", "TIMED_OUT", "ACTION_REQUIRED":
		return CheckBucketFail
	case "CANCELLED":
		return CheckBucketCancel
	default: // "EXPECTED", "REQUESTED", "WAITING", "QUEUED", "PENDING", "IN_PROGRESS", "STALE"
		return CheckBucketPending
	}
}
