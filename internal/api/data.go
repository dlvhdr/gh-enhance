package api

type Job struct {
	CompletedAt string
	Conclusion  string
	Name        string
	DatabaseId  int
	StartedAt   string
	Status      string
	Steps       []Step
}

type Step struct {
	CompletedAt string
	Conclusion  string
	Name        string
	Number      int
	StartedAt   string
	Status      string
}

type Run struct {
	Name     string
	Jobs     []Job
	Link     string
	Workflow string
}

type Check struct {
	Id       string
	State    string
	Name     string
	Workflow string
	Logs     string
	Link     string
}
