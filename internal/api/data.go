package api

type Step struct {
	CompletedAt string
	Conclusion  string
	Name        string
	Number      int
	StartedAt   string
	Status      string
}

type CheckRun struct {
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
	Link     string
}
