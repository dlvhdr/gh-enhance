package data

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
	State       api.Status
	Conclusion  api.Conclusion
	Name        string
	Workflow    string
	Event       string
	Logs        []LogsWithTime
	Link        string
	Steps       []api.Step
	StartedAt   time.Time
	CompletedAt time.Time
	Bucket      CheckBucket
	Kind        JobKind
}

type LogKind int

const (
	LogKindStepNone LogKind = iota
	LogKindStepStart
	LogKindGroupStart
	LogKindGroupEnd
	LogKindCommand
	LogKindError
	LogKindJobCleanup
	LogKindCompleteJob
)

type LogsWithTime struct {
	Log   string
	Time  time.Time
	Kind  LogKind
	Depth int
}

type JobKind int

const (
	JobKindCheckRun JobKind = iota
	JobKindGithubActions
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

func GetConclusionBucket(conclusion api.Conclusion) CheckBucket {
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
