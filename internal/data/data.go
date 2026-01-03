package data

import (
	"sort"
	"strings"
	"time"

	"github.com/dlvhdr/gh-enhance/internal/api"
)

// WorkflowRun holds all the the jobs that were part of it
// It is defined by a workflow file that defines the jobs to run
type WorkflowRun struct {
	Id        string
	Name      string
	Link      string
	Workflow  string
	Event     string
	Jobs      []WorkflowJob
	Bucket    CheckBucket
	StartedAt time.Time
	RunNumber int
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

	// A number that uniquely identifies this workflow run in its parent workflow.
	RunNumber int
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

// Order: failed -> in progress -> skipped -> neutral -> haven't started -> started at -> name
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

		if jobs[i].StartedAt.Equal(jobs[j].StartedAt) {
			return strings.Compare(jobs[i].Name, jobs[j].Name) < 0
		}

		return jobs[i].StartedAt.Before(jobs[j].StartedAt)
	})
}

func (job WorkflowJob) IsStatusInProgress() bool {
	return job.State == api.StatusInProgress
}

func SortRuns(runs []WorkflowRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Bucket == CheckBucketFail &&
			runs[j].Bucket != CheckBucketFail {
			return true
		}
		if runs[j].Bucket == CheckBucketFail &&
			runs[i].Bucket != CheckBucketFail {
			return false
		}

		if runs[i].Bucket == CheckBucketPending &&
			runs[j].Bucket != CheckBucketPending {
			return true
		}
		if runs[j].Bucket == CheckBucketPending &&
			runs[i].Bucket != CheckBucketPending {
			return false
		}

		if runs[i].Bucket == CheckBucketSkipping &&
			runs[j].Bucket != CheckBucketSkipping {
			return true
		}
		if runs[j].Bucket == CheckBucketSkipping &&
			runs[i].Bucket != CheckBucketSkipping {
			return false
		}

		if runs[i].Bucket == CheckBucketNeutral &&
			runs[j].Bucket != CheckBucketNeutral {
			return true
		}
		if runs[j].Bucket == CheckBucketNeutral &&
			runs[i].Bucket != CheckBucketNeutral {
			return false
		}

		return strings.Compare(runs[i].Name, runs[j].Name) == -1
	})
}
