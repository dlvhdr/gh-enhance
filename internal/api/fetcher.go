package api

import (
	"fmt"

	gh "github.com/cli/go-gh/v2/pkg/api"
)

type CheckRunOutput struct {
	Title   string
	Summary string
}

type CheckRunOutputResponse struct {
	Id     int
	Name   string
	Url    string
	Output CheckRunOutput
}

func FetchCheckRunOutput(repo string, runId string) (CheckRunOutputResponse, error) {
	client, err := gh.DefaultRESTClient()
	res := CheckRunOutputResponse{}
	if err != nil {
		return res, err
	}

	err = client.Get(fmt.Sprintf("repos/%s/check-runs/%s", repo, runId), &res)
	if err != nil {
		return res, err
	}

	return res, nil
}
