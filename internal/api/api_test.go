package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func createFakeListRepoActionRunsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/some/repo/actions/runs" {
			t.Fatalf("Incorrect path %s", r.URL.Path)
		}
		d, err := os.ReadFile("./testdata/repoPRs.json")
		if err != nil {
			t.Errorf("failed reading mock data file %v", err)
		}

		t.Log("unexpected url", r.URL)
		fmt.Fprint(w, string(d))
	}))
}

func TestFetchRepoPRs(t *testing.T) {
	svr := createFakeListRepoActionRunsServer(t)
	defer svr.Close()

	api := API{
		url:        svr.URL,
		httpClient: &http.Client{},
	}

	res, err := api.FetchRepoWorkflowRuns("some/repo", "cursor")
	if err != nil {
		t.Fatal(err)
	}

	if res.TotalCount != 1 {
		t.Fatalf("expected to get 1 runs, got %d", res.TotalCount)
	}

	if len(res.WorkflowRuns) != 1 {
		t.Fatalf("expected to get 1 runs, got %d", len(res.WorkflowRuns))
	}

	run := res.WorkflowRuns[0]
	if run.Id != 30433642 {
		t.Fatalf("expected to get run id of 30433642, got %d", run.Id)
	}
}
