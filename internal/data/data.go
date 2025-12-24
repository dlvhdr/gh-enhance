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
	CheckBucketNeutral
)

func GetConclusionBucket(conclusion api.Conclusion) CheckBucket {
	switch conclusion {
	case "SUCCESS":
		return CheckBucketPass
	case "SKIPPED":
		return CheckBucketSkipping
	case "NEUTRAL":
		return CheckBucketNeutral
	case "ERROR", "FAILURE", "TIMED_OUT", "ACTION_REQUIRED":
		return CheckBucketFail
	case "CANCELLED":
		return CheckBucketCancel
	default: // "EXPECTED", "REQUESTED", "WAITING", "QUEUED", "PENDING", "IN_PROGRESS", "STALE"
		return CheckBucketPending
	}
}

func (run WorkflowRun) SortJobs() {
	SortJobs(run.Jobs)
}

func SortJobs(jobs []WorkflowJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].Bucket == CheckBucketFail &&
			jobs[j].Bucket != CheckBucketFail {
			return true
		}
		if jobs[j].Bucket == CheckBucketFail &&
			jobs[i].Bucket != CheckBucketFail {
			return false
		}

		if jobs[i].State == api.StatusInProgress &&
			jobs[j].State != api.StatusInProgress {
			return true
		}
		if jobs[j].State == api.StatusInProgress &&
			jobs[i].State != api.StatusInProgress {
			return false
		}

		if jobs[i].Conclusion == api.ConclusionSkipped &&
			jobs[j].Conclusion != api.ConclusionSkipped {
			return true
		}
		if jobs[j].Conclusion == api.ConclusionSkipped &&
			jobs[i].Conclusion != api.ConclusionSkipped {
			return false
		}

		if jobs[i].Conclusion == api.ConclusionNeutral &&
			jobs[j].Conclusion != api.ConclusionNeutral {
			return true
		}
		if jobs[j].Conclusion == api.ConclusionNeutral &&
			jobs[i].Conclusion != api.ConclusionNeutral {
			return false
		}

		if jobs[i].StartedAt.IsZero() {
			return false
		}
		// if second job hasn't started yet, it should appear last
		if jobs[j].StartedAt.IsZero() {
			return true
		}

		return jobs[i].StartedAt.Before(jobs[j].StartedAt)
	})
}

func (job WorkflowJob) IsStatusInProgress() bool {
	return job.State == api.StatusInProgress
}
