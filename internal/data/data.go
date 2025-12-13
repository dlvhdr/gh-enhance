package data

import (
	"sort"
	"time"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

// WorkflowRun holds all the the jobs that were part of it
// It is defined by a workflow file that defines the jobs to run
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
	Title       string
	Workflow    string
	PendingEnv  string
	Event       string
	Logs        []LogsWithTime
	Link        string
	Steps       []api.Step
	StartedAt   time.Time
	CompletedAt time.Time
	Bucket      CheckBucket
	Kind        JobKind
	RunNumber   int
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

func (run WorkflowRun) SortJobs() {
	sort.Slice(run.Jobs, func(i, j int) bool {
		if run.Jobs[i].Bucket == CheckBucketFail &&
			run.Jobs[j].Bucket != CheckBucketFail {
			return true
		}
		if run.Jobs[j].Bucket == CheckBucketFail &&
			run.Jobs[i].Bucket != CheckBucketFail {
			return false
		}
		if run.Jobs[i].StartedAt.IsZero() {
			return false
		}
		if run.Jobs[j].StartedAt.IsZero() {
			return true
		}

		return run.Jobs[i].StartedAt.Before(run.Jobs[j].StartedAt)
	})
}
