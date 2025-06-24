package api

import "time"

type Step struct {
	CompletedAt time.Time
	Conclusion  string
	Name        string
	Number      int
	StartedAt   time.Time
	Status      string
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
	Jobs     []Job
}

type StepLogs string

type StepLogsWithTime struct {
	Log  string
	Time time.Time
}

type Job struct {
	Id          string
	State       string
	Name        string
	Workflow    string
	Event       string
	Logs        []StepLogsWithTime
	Loading     bool
	Link        string
	Steps       []Step
	StartedAt   time.Time
	CompletedAt time.Time
}
