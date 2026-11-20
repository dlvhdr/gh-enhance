package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	ghAPI "github.com/cli/go-gh/v2/pkg/api"
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

type rewriteTransport struct{ target *url.URL }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func TestFetchRepoPRs(t *testing.T) {
	svr := createFakeListRepoActionRunsServer(t)
	defer svr.Close()

	target, _ := url.Parse(svr.URL)
	rc, err := ghAPI.NewRESTClient(ghAPI.ClientOptions{
		Host:      "github.com",
		AuthToken: "test-token",
		Transport: rewriteTransport{target: target},
	})
	if err != nil {
		t.Fatal(err)
	}
	api := API{restClient: rc}

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

func TestWebURL(t *testing.T) {
	tests := []struct {
		name string
		host string
		path string
		want string
	}{
		{
			"unset host falls back to github.com",
			"",
			"o/r/pull/1",
			"https://github.com/o/r/pull/1",
		},
		{
			"github.com",
			"github.com",
			"o/r/pull/1",
			"https://github.com/o/r/pull/1",
		},
		{
			"custom GHE host",
			"github.example.com",
			"o/r/pull/1",
			"https://github.example.com/o/r/pull/1",
		},
		{
			"GHE host with port",
			"github.example.com:8443",
			"o/r/pull/1",
			"https://github.example.com:8443/o/r/pull/1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := API{host: tc.host}
			if got := a.webURL(tc.path); got != tc.want {
				t.Errorf("webURL(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRESTRequestFollowsHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{
			"github.com",
			"github.com",
			"https://api.github.com/repos/o/r/actions/runs",
		},
		{
			"custom GHE host",
			"github.example.com",
			"https://github.example.com/api/v3/repos/o/r/actions/runs",
		},
		{
			"GHE host with port",
			"github.example.com:8443",
			"https://github.example.com:8443/api/v3/repos/o/r/actions/runs",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				got = r.URL.String()
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(
						strings.NewReader(`{"total_count":0,"workflow_runs":[]}`),
					),
				}, nil
			})
			rc, err := ghAPI.NewRESTClient(ghAPI.ClientOptions{
				Host:      tc.host,
				AuthToken: "test-token",
				Transport: rt,
			})
			if err != nil {
				t.Fatal(err)
			}
			a := API{restClient: rc}
			if _, err := a.FetchRepoWorkflowRuns("o/r", ""); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("request URL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRerunRequests(t *testing.T) {
	tests := []struct {
		name     string
		call     func(a *API) error
		wantPath string
	}{
		{
			name:     "rerun a Job",
			call:     func(a *API) error { return a.ReRunJob("o/r", "1") },
			wantPath: "/repos/o/r/actions/jobs/1/rerun",
		},
		{
			name:     "rerun a whole run",
			call:     func(a *API) error { return a.ReRunRun("o/r", "2") },
			wantPath: "/repos/o/r/actions/runs/2/rerun",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				return &http.Response{
					StatusCode: http.StatusCreated,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			})

			rc, err := ghAPI.NewRESTClient(ghAPI.ClientOptions{
				Host:      "github.com",
				AuthToken: "test-token",
				Transport: rt,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := tc.call(&API{restClient: rc}); err != nil {
				t.Fatal(err)
			}

			if gotMethod != http.MethodPost {
				t.Errorf("method = %s, want %s", gotMethod, http.MethodPost)
			}

			if gotPath != tc.wantPath {
				t.Errorf("path = %s, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

func TestRerunReturnsHTTPErrors(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	rc, err := ghAPI.NewRESTClient(ghAPI.ClientOptions{
		Host:      "github.com",
		AuthToken: "test-token",
		Transport: rt,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := (&API{restClient: rc}).ReRunJob("o/r", "1"); err == nil {
		t.Error("expected an error for a 403 response, got nil")
	}
}
