package api

type Step struct {
	CompletedAt string
	Conclusion  string
	Name        string
	Number      int
	StartedAt   string
	Status      string
}

type JobWithSteps struct {
	CompletedAt string
	Conclusion  string
	DatabaseId  int
	Name        string
	StartedAt   string
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
	Jobs     []Job
}

type Job struct {
	Id       string
	State    string
	Name     string
	Workflow string
	Logs     string
	Loading  bool
	Link     string
	Steps    []Step
}
