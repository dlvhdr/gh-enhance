package api

import "time"

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
	case StatusCheckConclusionActionRequired, StatusCheckConclusionFailure, StatusCheckConclusionStartupFailure, StatusCheckConclusionTimedOut:
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
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Status      Status
}

type JobWithSteps struct {
	CompletedAt time.Time
	Conclusion  string
	DatabaseId  int
	Name        string
	StartedAt   time.Time
	Status      string
	Steps       []Step
}

type CheckRunJobsWithSteps struct {
	Jobs []JobWithSteps
}

type CheckRun struct {
	Id       string
	Name     string
	Link     string
	Workflow string
	Event    string
	Jobs     []StatusCheck
	Bucket   string // pass / skipping / fail / cancel / pending
}

type StepLogs string

type StepLogsWithTime struct {
	Log  string
	Time time.Time
}

type StatusCheck struct {
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
}
