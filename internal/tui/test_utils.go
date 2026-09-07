package tui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	gh "github.com/cli/go-gh/v2/pkg/api"
	"github.com/dlvhdr/gh-enhance/internal/api"
)

func setupLogger(t *testing.T) string {
	t.Helper()
	if _, debug := os.LookupEnv("DEBUG"); debug {
		wd, _ := os.Getwd()
		debugFile, _ := os.OpenFile("debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
		log.SetOutput(debugFile)
		log.SetLevel(log.DebugLevel)
		log.SetReportCaller(true)
		log.SetTimeFormat(time.Kitchen)
		log.SetColorProfile(colorprofile.TrueColor)
		debugDir := path.Join(wd, ".debug")
		os.Mkdir(debugDir, 0o700)

		runDir := path.Join(debugDir, fmt.Sprintf("run-%s-%d", t.Name(), time.Now().Unix()))
		err := os.Mkdir(runDir, 0o700)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("recording frames to %s", runDir)
		return runDir
	} else {
		log.SetOutput(io.Discard)
		log.SetLevel(log.FatalLevel)
	}

	return ""
}

// localRoundTripper is an http.RoundTripper that executes HTTP transactions
// by using handler directly, instead of going over an HTTP connection.
type localRoundTripper struct {
	handler http.Handler
}

func (l localRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func mustRead(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mustWrite(t *testing.T, w io.Writer, s string) {
	t.Helper()
	_, err := io.WriteString(w, s)
	if err != nil {
		panic(err)
	}
}

func makeMockClient(t *testing.T) api.API {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got http request", "url", req.URL.String(), "method", req.Method)
		switch {
		// https://api.github.com/repos/dlvhdr/gh-enhance/actions/jobs/44932094595
		case req.Method == http.MethodGet && strings.Contains(req.URL.String(), "/actions/jobs/"):
			mustWrite(t, w, "")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/graphql", func(w http.ResponseWriter, req *http.Request) {
		log.Debug("got graphql request", "url", req.URL.String(), "method", req.Method)
		body := ""
		if req.Method == http.MethodPost {
			body = mustRead(t, req.Body)
		}
		switch {
		case strings.Contains(body, "query FetchPR"):
			d, err := os.ReadFile("./testdata/fetchPR.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		case strings.Contains(body, "query FetchCheckRuns"):
			d, err := os.ReadFile("./testdata/fetchCheckRuns.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		case strings.Contains(body, "query FetchCheckRunSteps"):
			d, err := os.ReadFile("./testdata/fetchCheckRunSteps.json")
			if err != nil {
				t.Errorf("failed reading mock data file %v", err)
			}
			mustWrite(t, w, string(d))
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
	})
	client, err := gh.NewGraphQLClient(gh.ClientOptions{
		Transport: localRoundTripper{handler: mux},
		Host:      "localhost:3000",
		AuthToken: "fake-token",
	})
	if err != nil {
		t.Errorf("failed creating gh client %v", err)
	}

	tAPI := api.New()
	tAPI.SetGQLClient(client)
	return tAPI
}

func bytesContains(t *testing.T, bts []byte, str string) bool {
	t.Helper()
	return strings.Contains(ansi.Strip(string(bts)), str)
}

func waitForText(
	t *testing.T,
	tm *teatest.TestModel,
	framesDir string,
	text string,
	options ...teatest.WaitForOption,
) {
	t.Helper()
	teatest.WaitFor(t,
		tm.Output(),
		func(bts []byte) bool {
			contains := bytesContains(t, bts, text)
			now := time.Now()
			if _, debug := os.LookupEnv("DEBUG"); debug {
				f, err := os.CreateTemp(framesDir, fmt.Sprintf("gh-enhance-frame-%d", now.Unix()))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				fmt.Fprintf(f, "%s", string(bts))
				if contains {
					t.Log(
						"✅ text found",
						"file",
						f.Name(),
						"text",
						text,
					)
				} else {
					t.Log("❌ text not found", "file", f.Name(), "text", text)
				}
			}
			return contains
		},
		options...,
	)
}
